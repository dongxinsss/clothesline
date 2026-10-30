// Vector sources are the editable master. Requires sharp, plus Pillow for ICO/ICNS.
const fs = require('node:fs/promises');
const path = require('node:path');
const sharp = require('sharp');
const root = path.resolve(__dirname, '..');
async function main() {
  const app = await fs.readFile(path.join(root,'frontend/appicon.svg'));
  const tray = await fs.readFile(path.join(root,'frontend/tray.svg'));
  await sharp(app).resize(1024,1024).png().toFile(path.join(root,'frontend/appicon.png'));
  await sharp(tray).resize(72,72).png().toFile(path.join(root,'frontend/tray.png'));
  for (const size of [16,32,64,128,256,512,1024]) {
    await fs.mkdir(path.join(root,'build/icon-sizes'),{recursive:true});
    await sharp(app).resize(size,size).png().toFile(path.join(root,`build/icon-sizes/app-${size}.png`));
  }
}
main().catch(e=>{console.error(e);process.exitCode=1});
