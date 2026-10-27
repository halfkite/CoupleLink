import fs from 'node:fs';
import path from 'node:path';
import crypto from 'node:crypto';
import { fileURLToPath } from 'node:url';
import './check-contracts.mjs';
const root=path.resolve(path.dirname(fileURLToPath(import.meta.url)),'..');
const version=fs.readFileSync(path.join(root,'VERSION'),'utf8').trim();
const destination=path.join(root,'artifacts',version);
const sources=[
  [`apps/mobile/build/app/outputs/flutter-apk/app-release.apk`,`CoupleLink-Android-${version}.apk`],
  [`apps/band-vela/dist/app.couplelink.mobile.release.${version}.rpk`,`CoupleLink-Band10-${version}.rpk`],
  ['docs/ACCEPTANCE.md','安装与验收.md'],['docs/THIRD_PARTY.md','第三方声明.md'],
  ['apps/mobile/assets/fonts/OFL.txt','NotoSansSC-OFL.txt'],['docs/CHANGELOG.md','版本说明.md'],
];
for(const [source] of sources)if(!fs.existsSync(path.join(root,source)))throw new Error(`Missing artifact input: ${source}`);
fs.mkdirSync(destination,{recursive:true});
const hashes=[];
for(const [source,name]of sources){
  const bytes=fs.readFileSync(path.join(root,source));
  fs.writeFileSync(path.join(destination,name),bytes);
  hashes.push(`${crypto.createHash('sha256').update(bytes).digest('hex')}  ${name}`);
}
fs.writeFileSync(path.join(destination,'SHA256SUMS.txt'),hashes.join('\n')+'\n');
console.log(`Packaged ${version} to ${destination}`);
for(const [,name]of sources)console.log(name,fs.statSync(path.join(destination,name)).size);
