import { chromium } from 'playwright';
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
const root=path.resolve(path.dirname(fileURLToPath(import.meta.url)),'..');
const version=fs.readFileSync(path.join(root,'VERSION'),'utf8').trim();
const directory=path.join(root,'artifacts',version,'screenshots');
fs.mkdirSync(directory,{recursive:true});
const browser=await chromium.launch({executablePath:process.env.COUPLELINK_EDGE??'C:\\Program Files (x86)\\Microsoft\\Edge\\Application\\msedge.exe',
  headless:true,args:['--no-first-run']});
const page=await browser.newPage({viewport:{width:390,height:844},deviceScaleFactor:2,reducedMotion:'reduce'});
const errors=[];page.on('pageerror',error=>errors.push(error.message));
// Verify the preview without any external font, CDN, map or backend requests.
await page.route('**/*',route=>new URL(route.request().url()).hostname==='127.0.0.1'
  ?route.continue():route.abort());
await page.goto('http://127.0.0.1:5178',{waitUntil:'networkidle',timeout:60000});
await page.locator('flutter-view').waitFor({timeout:60000});
const placeholder=page.locator('flt-semantics-placeholder');
if(await placeholder.count())await placeholder.evaluate(element=>element.click());
await page.getByRole('button',{name:'首页',exact:true}).waitFor({timeout:30000});
const frame=()=>page.evaluate(()=>new Promise(resolve=>requestAnimationFrame(()=>requestAnimationFrame(resolve))));
await frame();
await page.screenshot({path:path.join(directory,'home-sakura.png')});
await page.getByRole('button',{name:'响铃',exact:true}).press('Enter');
await page.getByRole('button',{name:'本机试听',exact:true}).waitFor();
await frame();
await page.screenshot({path:path.join(directory,'feedback-ring.png')});
await page.getByRole('button',{name:'本机试听',exact:true}).press('Enter');
await page.getByText('当前设备或浏览器不支持此项本机反馈',{exact:true}).first().waitFor();
await frame();
await page.screenshot({path:path.join(directory,'feedback-browser-result.png')});
await page.getByRole('button',{name:'关闭',exact:true}).press('Enter');
console.log((await page.locator('body').innerText()).slice(0,1200));
for(const [label,file] of [['地图','map'],['消息','messages'],['时光','memories'],['我的','settings']]){
  await page.getByRole('button',{name:label,exact:true}).press('Enter');
  const element=await page.getByRole('button',{name:label,exact:true}).elementHandle();
  await page.waitForFunction(element=>element.getAttribute('aria-current')==='true',element);
  await frame();
  await page.screenshot({path:path.join(directory,`${file}.png`)});
}
for(const [name,file]of [['薰衣草','lavender'],['午夜心语','midnight']]){
  const theme=page.getByRole('button',{name,exact:true});
  await theme.press('Enter');
  const element=await theme.elementHandle();
  await page.waitForFunction(element=>element.getAttribute('aria-current')==='true',element);
  await frame();
  await page.screenshot({path:path.join(directory,`settings-${file}.png`)});
  await page.getByRole('button',{name:'首页',exact:true}).press('Enter');
  await frame();
  await page.screenshot({path:path.join(directory,`home-${file}.png`)});
  await page.getByRole('button',{name:'我的',exact:true}).press('Enter');
}
console.log(JSON.stringify({screenshots:directory,pageErrors:errors}));
await browser.close();
if(errors.length)process.exitCode=1;
