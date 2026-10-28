// Runs entirely on the development computer. No device tools or hardware access.
const path = require('node:path');
const assert = require('node:assert/strict');
const { spawn } = require('node:child_process');
const { chromium } = require('playwright');
const app = path.resolve(__dirname, '..');
const server = spawn(process.execPath, [path.join(app, 'node_modules/vite/bin/vite.js'), '--host', '127.0.0.1', '--port', '5181', '--strictPort'], { cwd: app, windowsHide: true, stdio: 'ignore' });
const origin = 'http://127.0.0.1:5181';
async function ready() { for(let n = 0; n < 60; n++) { if(server.exitCode !== null) throw Error('Local verification server exited'); try { if((await fetch(origin)).ok) return; } catch {} await new Promise(resolve => setTimeout(resolve, 100)); } throw Error('Local verification server not ready'); }
(async () => {
  await ready();
  const browser = await chromium.launch({ headless: true, ...(process.env.PLAYWRIGHT_EXECUTABLE_PATH ? { executablePath: process.env.PLAYWRIGHT_EXECUTABLE_PATH } : process.platform === 'win32' ? { executablePath: 'C:/Program Files (x86)/Microsoft/Edge/Application/msedge.exe' } : {}) });
  try {
    const page = await browser.newPage({ viewport: { width: 390, height: 844 } });
    const errors = []; page.on('pageerror', e => errors.push(e.message));
    await page.addInitScript(() => {
      window.bridgeCalls = []; window.feedbackResult = 'requested';
      window.androidBridge = {};
      window.Capacitor = {
        PluginHeaders: [{ name: 'CoupleDevice', methods: ['status', 'sessionRead', 'launch', 'feedback', 'notify', 'interaction', 'permission', 'diagnostics'].map(name => ({ name, rtype: 'promise' })) }],
        nativePromise: async (plugin, method, options) => {
          window.bridgeCalls.push({ plugin, method, options });
          if (method === 'status') return { running: false, notificationsEnabled: false };
          if (method === 'sessionRead') return { value: null };
          if (method === 'launch') return { kind: null };
          if (method === 'feedback') {
            if (window.feedbackResult === 'failed') throw { code: 'feedback_failed' };
            return { result: window.feedbackResult };
          }
          throw new Error('Unexpected native operation: ' + method);
        }
      };
    });
    await page.goto(origin); await page.waitForFunction(() => window.bridgeCalls.some(c => c.method === 'launch')); await page.locator('nav button').nth(4).click();
    for (const [result, label] of [
      ['requested', '已请求系统执行，请检查实际反馈'],
      ['unsupported', '此环境不支持该反馈'],
      ['suppressed', '本次反馈未执行，系统策略或设置限制了调用'],
      ['failed', '系统反馈执行失败，请检查设备设置。']
    ]) {
      await page.evaluate(result => { window.feedbackResult = result; }, result);
      await page.getByRole('button', { name: '测试震动', exact: true }).click();
      await page.locator('.banner').filter({ hasText: label }).waitFor({timeout: 5000}).catch(async e => { console.error(await page.evaluate(() => ({ calls: window.bridgeCalls, banners: [...document.querySelectorAll('.banner')].map(x => x.textContent) }))); throw e; });
    }
    await page.evaluate(() => { window.feedbackResult = 'requested'; });
    await page.getByRole('button', { name: '测试响铃', exact: true }).click();
    await page.locator('.banner').filter({ hasText: '已请求系统执行，请检查实际反馈' }).waitFor();
    const calls = await page.evaluate(() => window.bridgeCalls);
    assert.deepEqual(calls.filter(c => c.method === 'feedback').map(c => c.options), [{ sound: false }, { sound: false }, { sound: false }, { sound: false }, { sound: true }]);
    assert.equal(calls.some(c => ['notify', 'interaction', 'permission', 'diagnostics'].includes(c.method)), false);
    assert.deepEqual(errors, []);
    console.log('PASS: vibration button calls native feedback directly even with notifications disabled; no notification/interaction/permission calls; explicit requested/unsupported/suppressed/error states; ringing path retained. Hardware not tested.');
  } finally { await browser.close(); }
})().catch(e => { console.error(e.stack); process.exitCode = 1; }).finally(() => server.kill());
