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
  ['.tools/server-release/couplelink-windows-amd64.exe',`CoupleLink-Server-Windows-${version}.exe`],
  ['.tools/server-release/couplelink-linux-amd64',`CoupleLink-Server-Linux-${version}`],
  ['docs/SERVER.md','服务器部署.md'], ['docs/VALIDATION.md','验证记录.md'],
  ['docs/DOCKER-REPAIR.md','Docker故障记录.md'],['.tools/source-changes.diff','source-changes.diff'],
  ['deploy/compose.yaml','server-bundle/deploy/compose.yaml'],
  ['deploy/Caddyfile','server-bundle/deploy/Caddyfile'],
  ['deploy/.env.example','server-bundle/deploy/.env.example'],
  ['server/Dockerfile','server-bundle/server/Dockerfile'],
  ['server/go.mod','server-bundle/server/go.mod'],['server/go.sum','server-bundle/server/go.sum'],
  ['scripts/start-local-server.ps1','server-bundle/scripts/start-local-server.ps1'],
  ['scripts/enable-lan-access.ps1','server-bundle/scripts/enable-lan-access.ps1'],
  ...['cmd/couplelink/main.go','internal/api/server.go','internal/api/security.go','migrations/001_accounts.sql','migrations/embed.go']
    .map(file=>[`server/${file}`,`server-bundle/server/${file}`]),
];
for(const [source] of sources)if(!fs.existsSync(path.join(root,source)))throw new Error(`Missing artifact input: ${source}`);
fs.mkdirSync(destination,{recursive:true});
const hashes=[];
for(const [source,name]of sources){
  const bytes=fs.readFileSync(path.join(root,source));
  fs.mkdirSync(path.dirname(path.join(destination,name)),{recursive:true});
  fs.writeFileSync(path.join(destination,name),bytes);
  hashes.push(`${crypto.createHash('sha256').update(bytes).digest('hex')}  ${name}`);
}
fs.writeFileSync(path.join(destination,'SHA256SUMS.txt'),hashes.join('\n')+'\n');
console.log(`Packaged ${version} to ${destination}`);
for(const [,name]of sources)console.log(name,fs.statSync(path.join(destination,name)).size);
