import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
const root=path.resolve(path.dirname(fileURLToPath(import.meta.url)),'..');
const read=p=>fs.readFileSync(path.join(root,p),'utf8');
const json=p=>JSON.parse(read(p));
const version=read('VERSION').trim();
const match=/^1\.0-r([1-9]\d*)$/.exec(version);
assert.ok(match,'Version must remain 1.0-rN until the user changes the release line.');
const revision=Number(match[1]);
assert.equal(json('package.json').version,`1.0.0-r${revision}`);
for(const file of ['package-lock.json','apps/band-vela/package-lock.json']) {
  assert.equal(json(file).version,`1.0.0-r${revision}`);
  assert.equal(json(file).packages[''].version,`1.0.0-r${revision}`);
}
assert.ok(read('server/cmd/couplelink/main.go').includes(`var version = "${version}"`));
assert.ok(read('apps/mobile/pubspec.yaml').includes(`version: 1.0.0-r${revision}+${revision}`));
assert.ok(read('apps/mobile/lib/core/state.dart').includes(`appVersion = '${version}'`));
const manifest=json('apps/band-vela/src/manifest.json');
assert.equal(manifest.versionName,version);assert.equal(manifest.versionCode,revision);
assert.equal(manifest.package,'app.couplelink.mobile');assert.equal(manifest.config.designWidth,106);
assert.equal(Object.keys(manifest.router.pages).length,8);
for(const [route,config]of Object.entries(manifest.router.pages))assert.ok(fs.existsSync(path.join(root,`apps/band-vela/src/${route}/${config.component}.ux`)));
const zh=json('apps/mobile/assets/locales/zh_CN.json'),en=json('apps/mobile/assets/locales/en.json');
assert.deepEqual(Object.keys(zh).sort(),Object.keys(en).sort());
for(const key of Object.keys(zh)){
  assert.deepEqual(zh[key].match(/\{\w+\}/g)??[],en[key].match(/\{\w+\}/g)??[],`Placeholder mismatch: ${key}`);
}
const actions=json('packages/contracts/actions.json').actions;
assert.equal(new Set(actions.map(x=>x.id)).size,actions.length);
for(const action of actions){assert.ok(zh[action.labelKey]);assert.ok(action.expiresInSeconds>0);}
console.log(`${version}: version, 8 band routes, ${Object.keys(zh).length} bilingual keys and action contracts verified.`);
