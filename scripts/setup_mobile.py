#!/usr/bin/env python3
"""Install the pinned project toolchain on Linux x86_64, without global changes."""
import argparse
import json
import os
from pathlib import Path
import platform
import shutil
import subprocess
import sys
import tarfile
import tempfile
import zipfile

ROOT = Path(__file__).resolve().parents[1]
parser = argparse.ArgumentParser(description=__doc__)
parser.add_argument('--accept-android-license', action='store_true',
    help='Accept https://developer.android.com/studio#terms-and-conditions')
args = parser.parse_args()
if not args.accept_android_license:
    parser.error('Read the Android SDK terms, then explicitly pass --accept-android-license.')
if platform.system() != 'Linux' or platform.machine() != 'x86_64':
    raise SystemExit('This installer targets Linux x86_64. Use the pinned versions in docs/local-testing.md on other hosts.')
tools = ROOT / '.tools'
tools.mkdir(exist_ok=True)
lock = json.loads((ROOT / 'docs/mobile-toolchain-lock.json').read_text())

def run(*command, **options):
    return subprocess.run([str(c) for c in command], check=True, **options)

def download(url, target, checksum):
    run(sys.executable, ROOT / 'scripts/download_tool.py', url, target,
        '--sha256', checksum, '--workers', '32')

jdk = tools / 'jdk'
if not (jdk / 'bin/java').exists():
    image = 'eclipse-temurin@sha256:742daacb9316be4b6fc60258f6e4ac2a11882f6c597373c92290a57bfad0d2e0'
    run('docker', 'pull', image)
    container = subprocess.check_output(['docker', 'create', image], text=True).strip()
    try:
        with tempfile.TemporaryDirectory(dir=tools, prefix='jdk-extract-') as stage:
            run('docker', 'cp', container + ':/opt/java/openjdk', stage)
            shutil.move(str(Path(stage) / 'openjdk'), jdk)
    finally:
        run('docker', 'rm', container)
flutter = tools / 'flutter'
if not (flutter / 'bin/flutter').exists():
    archive = tools / 'flutter-3.41.7.tar.xz'
    download('https://storage.googleapis.com/flutter_infra_release/releases/stable/linux/' + archive.name,
        archive, lock['flutter_sha256'])
    with tempfile.TemporaryDirectory(dir=tools, prefix='flutter-extract-') as stage:
        with tarfile.open(archive) as source:
            source.extractall(stage, filter='data')
        if flutter.exists():
            raise SystemExit('Incomplete .tools/flutter exists; preserve or rename it before retrying setup.')
        shutil.move(str(Path(stage) / 'flutter'), flutter)
sdk = tools / 'android-sdk'
command_tools = sdk / 'cmdline-tools/latest'
if not (command_tools / 'bin/sdkmanager').exists():
    archive = tools / 'android-cli-ranged.zip'
    download('https://dl.google.com/android/repository/commandlinetools-linux-15859902_latest.zip',
        archive, lock['command_tools_sha256'])
    with tempfile.TemporaryDirectory(dir=tools, prefix='android-cli-extract-') as stage:
        with zipfile.ZipFile(archive) as source:
            source.extractall(stage)
        command_tools.parent.mkdir(parents=True, exist_ok=True)
        shutil.move(str(Path(stage) / 'cmdline-tools'), command_tools)
        for path in (command_tools / 'bin').iterdir():
            path.chmod(path.stat().st_mode | 0o111)
env = os.environ.copy()
env.update(JAVA_HOME=str(jdk), ANDROID_HOME=str(sdk), ANDROID_SDK_ROOT=str(sdk))
# This command records licences through Google's own tool after explicit consent.
run(command_tools / 'bin/sdkmanager', '--sdk_root=' + str(sdk), '--licenses',
    input='y\n' * 100, text=True, env=env)
run(sys.executable, ROOT / 'scripts/install_android_packages.py', '--accept-android-license', env=env)
run(sys.executable, ROOT / 'scripts/setup_gradle.py')
run(sys.executable, ROOT / 'scripts/mobile.py', '--version')
print('Ready. Run python3 scripts/mobile.py pub get --enforce-lockfile')
