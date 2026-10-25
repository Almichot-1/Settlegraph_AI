"""Select project tooling without changing the user's shared SDK configuration."""
import os
from pathlib import Path
import shutil

ROOT=Path(__file__).resolve().parents[1]
def environment():
 env=os.environ.copy()
 sdk=ROOT/'.tools/android-sdk'
 if (sdk/'platform-tools').exists():
  env['ANDROID_HOME']=str(sdk);env['ANDROID_SDK_ROOT']=str(sdk)
 jdk=ROOT/'.tools/jdk'
 if (jdk/'bin/java').exists():env['JAVA_HOME']=str(jdk)
 env['ANDROID_AVD_HOME']=str(ROOT/'.tools/avd');Path(env['ANDROID_AVD_HOME']).mkdir(parents=True,exist_ok=True)
 env['FLUTTER_SUPPRESS_ANALYTICS']='true'
 if (ROOT/'.tools/gradle').exists():env['GRADLE_USER_HOME']=str(ROOT/'.tools/gradle')
 paths=[]
 for key,suffix in [('JAVA_HOME','bin'),('ANDROID_HOME','platform-tools'),('ANDROID_HOME','emulator')]:
  if env.get(key):paths.append(str(Path(env[key])/suffix))
 flutter=ROOT/'.tools/flutter/bin/flutter'
 selected=env.get('FLUTTER_BIN') or (str(flutter) if flutter.exists() else shutil.which('flutter'))
 if not selected:raise RuntimeError('Flutter 3.41.7 is required; install it in .tools/flutter or set FLUTTER_BIN.')
 paths.insert(0,str(Path(selected).parent));env['PATH']=os.pathsep.join(paths+[env.get('PATH','')])
 return selected,env
