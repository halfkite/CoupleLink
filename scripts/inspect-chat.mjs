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
try{
 for(const person of people){
  const context=await browser.newContext({viewport:{width:390,height:844},deviceScaleFactor:2,reducedMotion:'reduce'}),page=await context.newPage();pages.push(page);
  page.on('pageerror',error=>errors.push(error.message));
  page.on('websocket',socket=>{
    console.log('WebSocket path:',new URL(socket.url()).pathname);
    socket.on('socketerror',error=>console.log('WebSocket transport error:',error));
  });
  await page.route('**/*',route=>new URL(route.request().url()).hostname==='127.0.0.1'?route.continue():route.abort());
  await page.goto('http://127.0.0.1:5178');await page.locator('flutter-view').waitFor();await page.locator('flt-semantics-placeholder').evaluate(e=>e.click());
  await login(page,person);
 }
 const [alice,bob]=pages;
 await visible(alice,'在线同步已连接');await visible(bob,'在线同步已连接');
 const body='真实文字聊天 · '+run,start=Date.now();await send(alice,body);await visible(bob,body);const latency=Date.now()-start;
 await alice.getByText(/ · 已送达$/).or(alice.getByLabel(/ · 已送达$/)).first().waitFor({timeout:15000});
 const received=await api('GET','/messages?couple_id='+couple,people[0].token);
 if(received.messages.find(row=>row.body===body)?.delivered!==true)throw Error('No real receiver receipt');
 await alice.screenshot({path:path.join(directory,'chat-real-delivered.png')});
 await bob.screenshot({path:path.join(directory,'chat-real-received.png')});
 await alice.route(origin+'/**',route=>offline?route.abort():route.continue());offline=true;
 const pending='断网后重启仍补发 · '+run;await send(alice,pending);await visible(alice,pending);
 await alice.getByLabel(new RegExp(pending+'.*待发送','s')).waitFor();
 await alice.screenshot({path:path.join(directory,'chat-offline-queue.png')});
 const storedBefore=await api('GET','/messages?couple_id='+couple,people[0].token);
 if(storedBefore.messages.some(row=>row.body===pending))throw Error('Offline message reached server before retry');
 // Browser sessions deliberately keep bearer tokens in memory. After reload,
 // re-authenticate, then verify the SQLite/IndexedDB queue uses the old ID.
 await alice.reload();await alice.locator('flutter-view').waitFor();const placeholder=alice.locator('flt-semantics-placeholder');if(await placeholder.count())await placeholder.evaluate(e=>e.click());
 offline=false;await login(alice,people[0]);await visible(bob,pending);
 const after=await api('GET','/messages?couple_id='+couple,people[0].token);
 if(after.messages.filter(row=>row.body===pending).length!==1)throw Error('Replayed offline message duplicated or lost');
 await alice.route(origin+'/**',route=>offline?route.abort():route.continue());offline=true;
 const canceled='解绑后不可补发 · '+run;await send(alice,canceled);await visible(alice,canceled);
 await alice.getByLabel(new RegExp(canceled+'.*待发送','s')).waitFor();
 await api('DELETE','/couples/current',people[1].token,{version:binding.couple.version});offline=false;
 await visible(alice,'先绑定情侣关系，即可开始真实聊天。');
 const denied=await fetch(origin+'/messages?couple_id='+couple,{headers:{Authorization:'Bearer '+people[0].token}});
 if(denied.status!==403)throw Error('Old relationship remained readable');
 if(errors.length)throw Error('Browser errors: '+errors.join('; '));
 console.log(JSON.stringify({realtimeConnected:true,firstMessageLatencyMs:latency,deliveredReceipt:true,offlineRestartExactlyOnce:true,unbindStopsRetry:true,pageErrors:errors}));
}catch(error){for(let i=0;i<pages.length;i++){
 await pages[i].screenshot({path:path.join(directory,`chat-test-failure-${i}.png`)}).catch(()=>{});
 fs.writeFileSync(path.join(root,'.tools',`chat-failure-dom-${i}.html`),await pages[i].content());
 }throw error;}
finally{await browser.close();}
