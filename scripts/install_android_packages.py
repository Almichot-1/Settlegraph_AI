#!/usr/bin/env python3
"""Install project Android packages from Google's published repository metadata.

Downloads use published checksums and resumable ranges. Accepting the SDK licence
is explicit; only the project's .tools/android-sdk directory is modified.
"""
import argparse
from concurrent.futures import ThreadPoolExecutor
import json
import os
from pathlib import Path
import shutil
import signal
import stat
import subprocess
import sys
import tempfile
import urllib.request
from xml.dom import minidom
from xml.sax.saxutils import quoteattr
import zipfile

ROOT=Path(__file__).resolve().parents[1]
p=argparse.ArgumentParser(description=__doc__);p.add_argument('--accept-android-license',action='store_true');a=p.parse_args()
if not a.accept_android_license:raise SystemExit('Read and accept https://developer.android.com/studio#terms-and-conditions before using --accept-android-license.')
sdk=ROOT/'.tools/android-sdk'
if not (sdk/'licenses/android-sdk-license').is_file():raise SystemExit('Run scripts/setup_mobile.py --accept-android-license to record the official SDK licences first.')
# Stop only a project SDK manager still downloading the same packages.
for process in Path('/proc').iterdir():
 if not process.name.isdigit():continue
 try:
  command=(process/'cmdline').read_bytes().decode().split('\0')
  if any('com.android.sdklib.tool.sdkmanager.SdkManagerCli' in c for c in command) and any(str(sdk) in c for c in command):os.kill(int(process.name),signal.SIGTERM)
 except (OSError,UnicodeDecodeError):pass
wanted={'platform-tools','platforms;android-36','platforms;android-35','cmake;3.22.1','build-tools;36.0.0','build-tools;35.0.0','emulator','ndk;28.2.13676358','system-images;android-35;default;x86_64'}
lock_path=ROOT/'docs/mobile-toolchain-lock.json'
if lock_path.exists():packages=json.loads(lock_path.read_text())['android_packages']
else:
 packages=[]
 for source in ['https://dl.google.com/android/repository/repository2-3.xml','https://dl.google.com/android/repository/sys-img/android/sys-img2-3.xml']:
  with urllib.request.urlopen(source,timeout=60) as r:doc=minidom.parseString(r.read())
  namespaces=' '.join(name+'='+quoteattr(doc.documentElement.getAttribute(name)) for name in doc.documentElement.attributes.keys() if name.startswith('xmlns'))
  licenses=''.join(node.toxml() for node in doc.getElementsByTagName('license'))
  for item in doc.getElementsByTagName('remotePackage'):
   name=item.getAttribute('path')
   if name not in wanted:continue
   channels=item.getElementsByTagName('channelRef')
   if channels and channels[0].getAttribute('ref')!='channel-0':continue
   for archive in item.getElementsByTagName('archive'):
    def text(tag):
     matches=archive.getElementsByTagName(tag)
     return matches[0].firstChild.data if matches and matches[0].firstChild else None
    if text('host-os') not in [None,'linux']:continue
    remote=item.cloneNode(deep=True);doc.renameNode(remote,None,'localPackage')
    for child in list(remote.childNodes):
     if getattr(child,'tagName',None) in ['archives','channelRef']:remote.removeChild(child)
    xml='<?xml version="1.0" encoding="UTF-8"?><localrepo:repository xmlns:localrepo="http://schemas.android.com/repository/android/common/02" '+namespaces+'>'+licenses+remote.toxml()+'</localrepo:repository>'
    checksum=archive.getElementsByTagName('checksum')[0]
    packages.append({'path':name,'url':source.rsplit('/',1)[0]+'/'+text('url'),'checksum':text('checksum'),'algorithm':checksum.getAttribute('type') or 'sha1','package_xml':xml})
 if {x['path'] for x in packages}!=wanted:raise SystemExit('Official repository is missing a required stable package; no package lock written.')
 lock_path.write_text(json.dumps({'flutter_version':'3.41.7','flutter_sha256':'f344d5057db52abc2a63cd3a7c7370957b7685d1fca5e5fbe2ce4dfe74657a79','command_tools_sha256':'4e4c464f145a7512b57d088ac6c278c03c9eea610886b35a5e0804e74eedf583','android_packages':packages},indent=2)+'\n')

def install(package):
 target=sdk.joinpath(*package['path'].split(';'))
 if (target/'package.xml').exists():
  (target/'package.xml').write_text(package['package_xml'])
  return
 archive=ROOT/'.tools/archives'/package['url'].rsplit('/',1)[1]
 subprocess.run([sys.executable,str(ROOT/'scripts/download_tool.py'),package['url'],str(archive),'--'+package['algorithm'],package['checksum'],'--workers','16'],check=True)
 with tempfile.TemporaryDirectory(dir=ROOT/'.tools',prefix='android-extract-') as tmp:
  stage=Path(tmp)
  with zipfile.ZipFile(archive) as z:
   for entry in z.infolist():
    relative=Path(entry.filename)
    if relative.is_absolute() or '..' in relative.parts:raise RuntimeError('Unsafe SDK archive path')
    dest=stage/relative;mode=entry.external_attr>>16
    if entry.is_dir():dest.mkdir(parents=True,exist_ok=True);continue
    dest.parent.mkdir(parents=True,exist_ok=True)
    if stat.S_ISLNK(mode):
     link=z.read(entry).decode()
     if not (dest.parent/link).resolve().is_relative_to(stage.resolve()):raise RuntimeError('Unsafe SDK archive symlink')
     dest.symlink_to(link)
    else:
     with z.open(entry) as source,dest.open('wb') as out:shutil.copyfileobj(source,out)
     if mode&0o777:dest.chmod(mode&0o777)
  roots=list(stage.iterdir())
  contents=roots[0] if len(roots)==1 and roots[0].is_dir() else stage
  target.parent.mkdir(parents=True,exist_ok=True)
  if target.exists():shutil.rmtree(target)
  shutil.copytree(contents,target,symlinks=True)
  (target/'package.xml').write_text(package['package_xml'])
 print('Installed official package:',package['path'],flush=True)
order={'platform-tools':0,'build-tools;35.0.0':1,'build-tools;36.0.0':1,'platforms;android-35':2,'platforms;android-36':2,'cmake;3.22.1':2,'emulator':3,'system-images;android-35;default;x86_64':4,'ndk;28.2.13676358':5}
with ThreadPoolExecutor(max_workers=2) as pool:list(pool.map(install,sorted(packages,key=lambda item:order[item['path']])))
print('Project Android SDK packages installed and checksums verified.')
