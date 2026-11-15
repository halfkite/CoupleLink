import { execFileSync } from 'node:child_process';
import fs from 'node:fs';
import path from 'node:path';
import crypto from 'node:crypto';
import { fileURLToPath } from 'node:url';

// One command produces the current release: web bundle, signed Android APK and
// the matching Go server binaries, all copied into artifacts/<VERSION>/.
const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const version = fs.readFileSync(path.join(root, 'VERSION'), 'utf8').trim();
const web = path.join(root, 'apps', 'mobile-web');
const android = path.join(web, 'android');
const server = path.join(root, 'server');
const go = process.env.GO || 'go';
const out = path.join(root, 'artifacts', version);

function run(file, args, cwd, env = {}) {
  process.stdout.write(`\n$ ${file} ${args.join(' ')}\n`);
  execFileSync(file, args, { cwd, stdio: 'inherit', env: { ...process.env, ...env } });
}

run(process.execPath, ['node_modules/vite/bin/vite.js', 'build'], web);
run(process.execPath, ['node_modules/@capacitor/cli/bin/capacitor', 'sync', 'android'], web);
if (process.platform === 'win32') run('cmd.exe', ['/d', '/s', '/c', 'gradlew.bat', '--no-daemon', 'assembleRelease'], android);
else run(path.join(android, 'gradlew'), ['--no-daemon', 'assembleRelease'], android);

fs.mkdirSync(path.join(out, 'server'), { recursive: true });
// The Go tool needs a writable work directory; keep it inside the project so a
// restricted TEMP/TMP (common on Windows) cannot break the release build.
const goTemp = path.join(root, '.tmp', 'gotmp');
fs.mkdirSync(goTemp, { recursive: true });
const serverBinaries = [
  ['windows', 'amd64', `couplelink-windows-amd64.exe`],
  ['linux', 'amd64', `couplelink-linux-amd64`],
];
for (const [GOOS, GOARCH, name] of serverBinaries) {
  run(go, ['build', '-trimpath', '-ldflags', '-s -w', '-o', path.join(out, 'server', name), './cmd/couplelink'], server,
    { GOOS, GOARCH, CGO_ENABLED: '0', GOTMPDIR: goTemp, TMP: goTemp, TEMP: goTemp });
}

const copies = [
  [path.join(android, 'app/build/outputs/apk/release/app-release.apk'), `CoupleLink-Android-${version}.apk`],
  ['docs/ACCEPTANCE.md', '安装与验收.md'],
  ['docs/FEEDBACK.md', '震动与反馈说明.md'],
  ['docs/CHANGELOG.md', '版本说明.md'],
  ['docs/SERVER.md', '服务器部署.md'],
];
const hashes = [];
for (const [source, name] of copies) {
  const file = path.isAbsolute(source) ? source : path.join(root, source);
  if (!fs.existsSync(file)) {
    console.warn(`skipping missing input: ${source}`);
    continue;
  }
  const bytes = fs.readFileSync(file);
  fs.writeFileSync(path.join(out, name), bytes);
  hashes.push(`${crypto.createHash('sha256').update(bytes).digest('hex')}  ${name}`);
}
for (const [, , name] of serverBinaries) {
  const bytes = fs.readFileSync(path.join(out, 'server', name));
  hashes.push(`${crypto.createHash('sha256').update(bytes).digest('hex')}  server/${name}`);
}
fs.writeFileSync(path.join(out, 'SHA256SUMS.txt'), hashes.join('\n') + '\n');
console.log(`\nRelease ${version} written to ${out}`);
console.log(`Server default listen address: 0.0.0.0:30710, admin page http://127.0.0.1:30710/admin`);