import fs from 'node:fs';
import path from 'node:path';
import zlib from 'node:zlib';
import { fileURLToPath } from 'node:url';
const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
function crc(buffer) {
  let result = 0xffffffff;
  for (const byte of buffer) { result ^= byte; for (let i=0;i<8;i++) result = (result>>>1)^((result&1)?0xedb88320:0); }
  return (result^0xffffffff)>>>0;
}
function chunk(type,data) {
  const name=Buffer.from(type), length=Buffer.alloc(4),checksum=Buffer.alloc(4);
  length.writeUInt32BE(data.length);checksum.writeUInt32BE(crc(Buffer.concat([name,data])));
  return Buffer.concat([length,name,data,checksum]);
}
function png(size,pixel) {
  const header=Buffer.alloc(13);header.writeUInt32BE(size,0);header.writeUInt32BE(size,4);header[8]=8;header[9]=6;
  const scan=Buffer.alloc(size*(size*4+1));
  for(let y=0;y<size;y++) for(let x=0;x<size;x++) {
    const color=[0,0,0,0];
    for(let sy=0;sy<2;sy++)for(let sx=0;sx<2;sx++) {
      const p=pixel((x+(sx+.5)/2)/size,(y+(sy+.5)/2)/size);
      for(let k=0;k<4;k++)color[k]+=p[k]/4;
    }
    for(let k=0;k<4;k++)scan[y*(size*4+1)+1+x*4+k]=Math.round(color[k]);
  }
  return Buffer.concat([Buffer.from([137,80,78,71,13,10,26,10]),chunk('IHDR',header),chunk('IDAT',zlib.deflateSync(scan)),chunk('IEND',Buffer.alloc(0))]);
}
function heart(x,y,cx,cy,scale) {
  const a=(x-cx)/scale,b=(cy-y)/scale;
  return Math.pow(a*a+b*b-1,3)-a*a*b*b*b<=0;
}
function icon(x,y){
  const dx=Math.max(.19-x,0,x-.81),dy=Math.max(.19-y,0,y-.81);
  if(dx*dx+dy*dy>.19*.19)return [0,0,0,0];
  const t=(x+y)/2;
  let c=[Math.round(238-61*t),Math.round(170-19*t),Math.round(192+31*t),255];
  if(heart(x,y,.43,.48,.22))c=[255,249,251,255];
  if(heart(x,y,.65,.62,.15))c=[203,101,142,255];
  return c;
}
function pet(x,y){
  const circle=(cx,cy,rx,ry)=>((x-cx)/rx)**2+((y-cy)/ry)**2<1;
  let c=[0,0,0,0];
  if(circle(.5,.87,.33,.045))c=[74,57,87,180];
  if(circle(.5,.62,.29,.27)||circle(.5,.41,.36,.26)||
    (y>.12&&y<.4&&x>.17&&x<.38&&x>.17+(y-.12)*.2)||
    (y>.12&&y<.4&&x>.62&&x<.83&&x<.83-(y-.12)*.2))c=[255,247,236,255];
  if(circle(.28,.46,.055,.03)||circle(.73,.46,.055,.03))c=[239,170,190,255];
  if(circle(.37,.4,.018,.024)||circle(.63,.4,.018,.024))c=[80,62,76,255];
  if(circle(.5,.49,.028,.018))c=[202,126,157,255];
  if(y>.63&&y<.68&&x>.28&&x<.73)c=[229,139,182,255];
  if(circle(.5,.69,.033,.034))c=[232,194,120,255];
  return c;
}
function write(relative,data){const target=path.join(root,relative);fs.mkdirSync(path.dirname(target),{recursive:true});fs.writeFileSync(target,data);}
write('apps/band-vela/src/common/icon.png',png(96,icon));
write('apps/band-vela/src/common/pet.png',png(120,pet));
for(const [density,size]of Object.entries({mdpi:48,hdpi:72,xhdpi:96,xxhdpi:144,xxxhdpi:192}))
  write(`apps/mobile/android/app/src/main/res/mipmap-${density}/ic_launcher.png`,png(size,icon));
for(const size of [192,512])write(`apps/mobile/web/icons/Icon-${size}.png`,png(size,icon));
write('apps/mobile/web/favicon.png',png(48,icon));
console.log('Generated original CoupleLink icons and band pet assets.');
