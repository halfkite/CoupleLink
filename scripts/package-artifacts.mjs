import fs from 'node:fs';
import path from 'node:path';
import crypto from 'node:crypto';
import { fileURLToPath } from 'node:url';
import './check-contracts.mjs';
const root=path.resolve(path.dirname(fileURLToPath(import.meta.url)),'..');
const version=fs.readFileSync(path.join(root,'VERSION'),'utf8').trim();
const outputLabel=process.argv[2] || version;
if(!new RegExp(`^${version.replaceAll('.', '\\.')}(-[a-z0-9-]+)?$`).test(outputLabel))throw new Error('Artifact folder must use the current version and an optional local correction label');
const destination=path.join(root,'artifacts',outputLabel);
const apkMetadata=JSON.parse(fs.readFileSync(path.join(root,'apps/mobile-web/android/app/build/outputs/apk/release/output-metadata.json'),'utf8'));
if(apkMetadata.applicationId!=='app.couplelink.mobile'||apkMetadata.elements.length!==1||apkMetadata.elements[0].versionName!==version||apkMetadata.elements[0].versionCode!==Number(version.split('-r')[1]))throw new Error('Android release metadata does not match VERSION');
const sources=[
  [`apps/mobile-web/android/app/build/outputs/apk/release/app-release.apk`,`CoupleLink-Android-${version}.apk`],
  [`apps/band-vela/dist/app.couplelink.mobile.release.${version}.rpk`,`CoupleLink-Band10-${version}.rpk`],
  ['docs/ACCEPTANCE.md','安装与验收.md'],['docs/THIRD_PARTY.md','第三方声明.md'],
  ['apps/mobile/assets/fonts/OFL.txt','NotoSansSC-OFL.txt'],['docs/CHANGELOG.md','版本说明.md'],
  ['.tools/server-release/couplelink-windows-amd64.exe',`CoupleLink-Server-Windows-${version}.exe`],
  ['.tools/server-release/couplelink-linux-amd64',`CoupleLink-Server-Linux-${version}`],
  ['docs/SERVER.md','服务器部署.md'], ['docs/VALIDATION.md','验证记录.md'],
  ['docs/DOCKER-REPAIR.md','Docker故障记录.md'],['.tools/source-changes.diff','source-changes.diff'],
  ['docs/CHAT.md','文字聊天说明.md'],
  ['docs/INTERACTIONS.md','互动说明.md'],
  ['docs/NOTIFICATIONS.md','通知与服务器地址.md'],
  ['docs/WEB-ANDROID.md','Web安卓客户端说明.md'],
  ['docs/licenses/capacitor-LICENSE.txt','third-party/capacitor-LICENSE.txt'],
  ['docs/licenses/vue-LICENSE.txt','third-party/vue-LICENSE.txt'],
  ['docs/licenses/Apache-2.0.txt','third-party/Apache-2.0.txt'],
  ['apps/mobile/web/drift_worker.js','third-party/drift_worker.js'],
  ['apps/mobile/web/sqlite3.wasm','third-party/sqlite3.wasm'],
  ['docs/licenses/drift-LICENSE.txt','third-party/drift-LICENSE.txt'],
  ['docs/licenses/sqlite3-dart-LICENSE.txt','third-party/sqlite3-dart-LICENSE.txt'],
  ['docs/licenses/centrifuge-LICENSE.txt','third-party/centrifuge-LICENSE.txt'],
  ['docs/licenses/drift_flutter-LICENSE.txt','third-party/drift_flutter-LICENSE.txt'],
  ['docs/licenses/flutter_foreground_task-LICENSE.txt','third-party/flutter_foreground_task-LICENSE.txt'],
  ['docs/licenses/flutter_local_notifications-LICENSE.txt','third-party/flutter_local_notifications-LICENSE.txt'],
  ['deploy/compose.yaml','server-bundle/deploy/compose.yaml'],
  ['deploy/Caddyfile','server-bundle/deploy/Caddyfile'],
  ['deploy/centrifugo.json','server-bundle/deploy/centrifugo.json'],
  ['deploy/.env.example','server-bundle/deploy/.env.example'],
  ['server/Dockerfile','server-bundle/server/Dockerfile'],
  ['server/go.mod','server-bundle/server/go.mod'],['server/go.sum','server-bundle/server/go.sum'],
  ['scripts/start-local-server.ps1','server-bundle/scripts/start-local-server.ps1'],
  ['scripts/enable-lan-access.ps1','server-bundle/scripts/enable-lan-access.ps1'],
  ['docs/MEMORIES.md','日期纪念日与聊天搜索.md'],
  ['docs/FEEDBACK.md','直接震动修复说明.md'],
  ['.tools/direct-feedback.diff','direct-feedback.diff'],
  ...['cmd/couplelink/main.go','internal/api/server.go','internal/api/security.go','internal/api/chat.go','internal/api/interactions.go','internal/api/realtime.go','internal/api/memories.go','migrations/001_accounts.sql','migrations/002_chat.sql','migrations/003_interactions.sql','migrations/004_memories.sql','migrations/embed.go']
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
