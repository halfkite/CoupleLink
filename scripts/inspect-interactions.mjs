import {chromium} from 'playwright';
import fs from 'node:fs';
import path from 'node:path';
import crypto from 'node:crypto';
import {fileURLToPath} from 'node:url';
const root=path.resolve(path.dirname(fileURLToPath(import.meta.url)),'..');
const version=fs.readFileSync(path.join(root,'VERSION'),'utf8').trim();
const directory=path.join(root,'artifacts',version,'screenshots');fs.mkdirSync(directory,{recursive:true});
const origin=process.env.COUPLELINK_TEST_API_ORIGIN;
if(!origin||new URL(origin).hostname!=='127.0.0.1')throw Error('Use an isolated loopback test API.');
const password='12345678',run=Date.now().toString(36),errors=[];
async function api(method,route,token,body){
 const response=await fetch(origin+route,{method,headers:{...(token?{Authorization:'Bearer '+token}:{}),...(body?{'Content-Type':'application/json'}:{})},...(body?{body:JSON.stringify(body)}:{})});
 const data=await response.json();if(!response.ok)throw Error(route+' HTTP '+response.status+' '+data.code);return data;
}
const people=[];
for(const name of ['alice','bob']){
 const username='chat_'+name+'_'+run;
 const account=await api('POST','/auth/register',null,{username,password,display_name:name==='alice'?'测试甲':'测试乙',device_id:crypto.randomUUID(),device_label:'Isolated setup',platform:'test'});
 people.push({username,token:account.token});
}
const invitation=await api('POST','/couples/invitations',people[0].token);
const binding=await api('POST','/couples/join',people[1].token,{token:invitation.token,start_at:'2025-10-26',timezone:'Asia/Hong_Kong'});
const couple=binding.couple.id;
const browser=await chromium.launch({executablePath:process.env.COUPLELINK_EDGE??'C:/Program Files (x86)/Microsoft/Edge/Application/msedge.exe',headless:true});
const pages=[];let offline=false;
async function enter(page,label,value){
 const field=page.getByRole('textbox',{name:label,exact:true});
 await field.click({force:label==='写下想说的话'});
 await page.keyboard.press('ControlOrMeta+A');await page.keyboard.insertText(value);await page.keyboard.press('Tab');
}
async function login(page,person){
 await page.getByRole('button',{name:'我的',exact:true}).press('Enter');
 await page.getByRole('button',{name:/连接服务器 \/ 登录 \/ 情侣绑定/}).press('Enter');
 await enter(page,'服务器地址',origin);await enter(page,'账号名',person.username);await enter(page,'密码',password);
 await page.getByRole('button',{name:'登录',exact:true}).press('Enter');await page.getByRole('button',{name:'刷新同步',exact:true}).waitFor();
 await page.getByRole('button',{name:'返回',exact:true}).press('Enter');await page.getByRole('button',{name:'消息',exact:true}).press('Enter');
 await page.getByRole('textbox',{name:'写下想说的话',exact:true}).waitFor();
 await page.waitForFunction(()=>document.querySelector('input[aria-label="写下想说的话"],textarea[aria-label="写下想说的话"]')?.getAttribute('aria-disabled')!=='true');
}
async function visible(page,text){
 await page.bringToFront();
 await page.getByText(text,{exact:false}).or(page.getByLabel(text,{exact:false})).first().waitFor({timeout:15000});
}
async function send(page,body){await enter(page,'写下想说的话',body);await page.getByRole('button',{name:'发送',exact:true}).press('Enter');}
try {
 for (const person of people) {
  const context=await browser.newContext({viewport:{width:390,height:844},deviceScaleFactor:2,reducedMotion:'reduce'}),page=await context.newPage();pages.push(page);
  page.on('pageerror',error=>errors.push(error.message));
  await page.route('**/*',route=>new URL(route.request().url()).hostname==='127.0.0.1'?route.continue():route.abort());
  await page.goto('http://127.0.0.1:5178');await page.locator('flutter-view').waitFor();await page.locator('flt-semantics-placeholder').evaluate(e=>e.click());await login(page,person);
  await page.getByRole('button',{name:'首页',exact:true}).press('Enter');
 }
 const [alice,bob]=pages;
 await alice.bringToFront();await alice.getByRole('button',{name:'戳一下',exact:true}).press('Enter');
 await visible(bob,'收到 TA 的互动');await visible(bob,'戳一下');await visible(bob,'设备不支持反馈');
 await bob.screenshot({path:path.join(directory,'interaction-received.png')});
 await bob.getByRole('button',{name:'完成',exact:true}).press('Enter');
 let rows=[];for(let i=0;i<30;i++){rows=(await api('GET','/interactions?couple_id='+couple,people[0].token)).interactions;if(rows[0]?.result==='unsupported')break;await new Promise(resolve=>setTimeout(resolve,200));}
 if(rows[0]?.result!=='unsupported')throw Error('Receiver feedback result was not recorded');
 const poke=rows[0];const duplicate=await api('POST','/interactions',people[0].token,{id:poke.id,couple_id:couple,action:'poke'});if(duplicate.id!==poke.id)throw Error('Duplicate interaction changed ID');
 await alice.bringToFront();await alice.getByRole('button',{name:'响铃',exact:true}).press('Enter');
 await visible(bob,'收到 TA 的互动');await visible(bob,'响铃');await visible(bob,'接收方设置或系统阻止了声音或振动');
 await bob.screenshot({path:path.join(directory,'interaction-ring-muted.png')});
 rows=[];for(let i=0;i<30;i++){rows=(await api('GET','/interactions?couple_id='+couple,people[0].token)).interactions;if(rows[0]?.action==='ring'&&rows[0]?.result==='suppressed')break;await new Promise(resolve=>setTimeout(resolve,200));}
 if(rows[0]?.result!=='suppressed')throw Error('Ring permission was not respected');
 if(errors.length)throw Error('Browser errors: '+errors.join('; '));
 console.log(JSON.stringify({pokeReceived:true,duplicateIdempotent:true,feedbackUnsupportedReported:true,ringDefaultSuppressed:true,pageErrors:errors}));
} catch(error){for(let i=0;i<pages.length;i++)await pages[i].screenshot({path:path.join(directory,'interaction-failure-'+i+'.png')}).catch(()=>{});throw error;}
finally {await browser.close();}
