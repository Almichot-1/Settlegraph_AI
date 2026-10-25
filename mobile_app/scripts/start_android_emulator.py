#!/usr/bin/env python3
"""Open a visible emulator and wait until Android properties are readable.

Cold boot and software graphics avoid broken snapshots/graphics contexts on
Mac development hosts. This script never wipes emulator data.
"""
import argparse
import sys
import os
from pathlib import Path
import subprocess
import tempfile
import time

ROOT=Path(__file__).resolve().parents[2]
sys.path.insert(0,str(ROOT/'scripts'))
from mobile_env import environment
_,selected_env=environment()
os.environ.update(selected_env)
parser = argparse.ArgumentParser(description=__doc__)
parser.add_argument('--avd', default='SettleGraph_Review')
parser.add_argument('--port', type=int, default=5554)
parser.add_argument('--headless',action='store_true')
parser.add_argument('--timeout', type=int, default=180)
args = parser.parse_args()
if args.port % 2 or not 5554 <= args.port <= 5682:
    parser.error('--port must be an even emulator port between 5554 and 5682')
sdk = Path(os.environ.get('ANDROID_SDK_ROOT') or os.environ.get('ANDROID_HOME') or Path.home() / 'Library/Android/sdk')
adb = sdk / 'platform-tools/adb'
emulator = sdk / 'emulator/emulator'
if not adb.is_file() or not emulator.is_file():
    raise SystemExit('Android SDK not found. Set ANDROID_SDK_ROOT to your SDK directory.')
avd_home=Path(os.environ['ANDROID_AVD_HOME'])
if not (avd_home/(args.avd+'.ini')).exists():
    manager=sdk/'cmdline-tools/latest/bin/avdmanager'
    if not manager.exists():raise SystemExit('Install Android command-line tools before creating the project emulator.')
    subprocess.run([str(manager),'create','avd','-n',args.avd,'-k',('system-images;android-35;default;x86_64' if (sdk/'system-images/android-35/default/x86_64').exists() else 'system-images;android-35;google_apis;x86_64'),'-d','pixel_5'],input='no\n',text=True,check=True)
serial = f'emulator-{args.port}'
log_path = Path(tempfile.gettempdir()) / 'settlegraph-emulator.log'

def run(*command, timeout=5):
    try:
        return subprocess.run([str(c) for c in command], capture_output=True, text=True, timeout=timeout)
    except subprocess.TimeoutExpired:
        return None

run(adb, 'start-server', timeout=15)
processes = run('ps', '-Ao', 'args=')
existing = processes and any(
    ('qemu-system' in line or '/emulator/emulator ' in line)
    and f'-avd {args.avd} ' in line and f'-port {args.port}' in line
    for line in processes.stdout.splitlines()
)
child = None
if not existing:
    with log_path.open('w') as log:
        child = subprocess.Popen([
            str(emulator), '-avd', args.avd, '-port', str(args.port),
            '-no-snapshot-load', '-gpu', 'swiftshader', '-feature', '-Vulkan',
            '-no-audio', '-no-boot-anim', '-memory','1536','-cores','2',
            *(['-no-window'] if args.headless else []),
        ], stdin=subprocess.DEVNULL, stdout=log, stderr=subprocess.STDOUT,
            start_new_session=True)
    print(f'Opening {args.avd}. Startup log: {log_path}', flush=True)
else:
    print(f'Waiting for existing {args.avd} to finish booting…', flush=True)

started = time.monotonic()
last_progress = started
while time.monotonic() - started < args.timeout:
    if child is not None and child.poll() is not None:
        raise SystemExit(f'Emulator exited. Read {log_path} for details.')
    boot = run(adb, '-s', serial, 'shell', 'getprop', 'sys.boot_completed')
    if boot and boot.returncode == 0 and boot.stdout.strip() == '1':
        version = run(adb, '-s', serial, 'shell', 'getprop', 'ro.build.version.sdk')
        abi = run(adb, '-s', serial, 'shell', 'getprop', 'ro.product.cpu.abi')
        if version and abi and version.returncode == abi.returncode == 0 and version.stdout.strip().isdigit() and abi.stdout.strip():
            print(f'Ready: {serial}, Android API {version.stdout.strip()}, {abi.stdout.strip()}.', flush=True)
            print('Run Flutter only after this Ready message.', flush=True)
            break
    if time.monotonic() - last_progress >= 15:
        print('Android is still booting; waiting for ADB properties…', flush=True)
        last_progress = time.monotonic()
    time.sleep(1)
else:
    raise SystemExit('Android did not become ready. Close its emulator window and rerun this script; no data has been wiped.')
