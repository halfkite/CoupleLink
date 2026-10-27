import {chromium} from 'playwright';
import fs from 'node:fs';
import path from 'node:path';
import {fileURLToPath} from 'node:url';
const root=path.resolve(path.dirname(fileURLToPath(import.meta.url)),'..');
const version=fs.readFileSync(path.join(root,'VERSION'),'utf8').trim();
const directory=path.join(root,'artifacts',version,'screenshots');
const password='isolated UI integration password';
const apiOrigin=process.env.COUPLELINK_TEST_API_ORIGIN;
if(!apiOrigin||new URL(apiOrigin).hostname!=='127.0.0.1')throw Error('Set COUPLELINK_TEST_API_ORIGIN to an isolated loopback test API; do not use a real personal server.');
const browser=await chromium.launch({executablePath:process.env.COUPLELINK_EDGE??'C:/Program Files (x86)/Microsoft/Edge/Application/msedge.exe',headless:true});
const errors=[];
async function enter(page,label,value) {
  await page.getByRole('textbox',{name:label,exact:true}).click();
  await page.keyboard.press('ControlOrMeta+A');
  await page.keyboard.insertText(value);
  await page.keyboard.press('Tab');
}
try {
  const pages=[];
  for(const [username,display] of [['qa_alice','测试甲'],['qa_bob','测试乙']]) {
    const context=await browser.newContext({viewport:{width:390,height:844},deviceScaleFactor:2,reducedMotion:'reduce'});
    const page=await context.newPage(); pages.push(page);
    page.on('response',async response=>{if(response.url().startsWith(apiOrigin)){console.log(response.request().method(),new URL(response.url()).pathname,response.status());if(response.status()>=400)console.log('API error:',(await response.json()).code);}});
    page.on('pageerror',error=>errors.push(error.message));
    await page.route('**/*',route=>new URL(route.request().url()).hostname==='127.0.0.1'?route.continue():route.abort());
    await page.goto('http://127.0.0.1:5178');
    await page.locator('flutter-view').waitFor();
    await page.locator('flt-semantics-placeholder').evaluate(element=>element.click());
    await page.getByRole('button',{name:'我的',exact:true}).press('Enter');
    await page.getByRole('button',{name:/连接服务器 \/ 登录 \/ 情侣绑定/}).press('Enter');
    if(username==='qa_alice') {
      await page.getByRole('checkbox',{name:'注册',exact:true}).press('Enter');
      await page.getByRole('textbox',{name:'显示昵称',exact:true}).waitFor();
      await page.evaluate(()=>new Promise(resolve=>requestAnimationFrame(()=>requestAnimationFrame(resolve))));
      await page.screenshot({path:path.join(directory,'registration-no-setup.png')});
      await page.getByRole('checkbox',{name:'登录',exact:true}).press('Enter');
    }
    await enter(page,'服务器地址',apiOrigin);
    await enter(page,'账号名',username);
    await enter(page,'密码',password);
    await page.getByRole('button',{name:'登录',exact:true}).press('Enter');
    try { await Promise.race([
      page.getByText('账号或密码不正确',{exact:true}).first().waitFor(),
      page.getByRole('button',{name:'刷新同步',exact:true}).waitFor(),
    ]); } catch (error) { console.log(await page.locator('body').innerText()); await page.screenshot({path:path.join(directory,'account-test-failure.png')}); throw error; }
    if(await page.getByText('账号或密码不正确',{exact:true}).count()) {
      await page.getByRole('checkbox',{name:'注册',exact:true}).press('Enter');
      await enter(page,'显示昵称',display);
      await page.getByRole('button',{name:'注册',exact:true}).press('Enter');
    }
    await page.mouse.move(220,350);
    await page.mouse.wheel(0,-2400);
    try { await page.getByRole('button',{name:'刷新同步',exact:true}).waitFor(); }
    catch(error) { console.log(await page.locator('body').innerText()); await page.screenshot({path:path.join(directory,'account-test-failure.png')}); throw error; }
    await page.getByRole('button',{name:'生成配对邀请',exact:true}).waitFor();
  }
  const [alice,bob]=pages;
  const invitationResponse=alice.waitForResponse(response=>response.url().endsWith('/couples/invitations')&&response.request().method()==='POST');
  await alice.getByRole('button',{name:'生成配对邀请',exact:true}).press('Enter');
  const invitation=await (await invitationResponse).json();
  if(typeof invitation.token!=='string')throw Error('Invitation response missing token');
  await enter(bob,'粘贴对方的邀请码',invitation.token);
  bob.on('request',request=>{if(request.url().endsWith('/couples/join')){const data=request.postDataJSON();console.log('Join fixture validation:',{tokenMatches:data.token===invitation.token,tokenLength:data.token?.length,start:data.start_at,zone:data.timezone});}});
  await bob.getByRole('button',{name:'确认并绑定',exact:true}).press('Enter');
  await bob.getByRole('button',{name:'解除情侣绑定',exact:true}).waitFor();
  await alice.getByRole('button',{name:'刷新同步',exact:true}).press('Enter');
  await alice.getByRole('button',{name:'解除情侣绑定',exact:true}).waitFor();
  await alice.evaluate(()=>new Promise(resolve=>requestAnimationFrame(()=>requestAnimationFrame(resolve))));
  await alice.screenshot({path:path.join(directory,'account-real-paired.png')});
  await bob.getByRole('button',{name:'解除情侣绑定',exact:true}).press('Enter');
  await bob.getByRole('button',{name:'确认',exact:true}).press('Enter');
  await bob.getByRole('button',{name:'生成配对邀请',exact:true}).waitFor();
  await alice.getByRole('button',{name:'刷新同步',exact:true}).press('Enter');
  await alice.getByRole('button',{name:'生成配对邀请',exact:true}).waitFor();
  for(const page of pages) {
    await page.getByRole('button',{name:'退出登录',exact:true}).press('Enter');
    await page.getByRole('button',{name:'登录',exact:true}).waitFor();
  }
  if(errors.length)throw Error(errors.join('; '));
  console.log('Real Flutter Web → Go → PostgreSQL verified: two-account login/register, invite, pairing, peer refresh, unbind and logout. No private credentials printed.');
} finally { await browser.close(); }
