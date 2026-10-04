#!/usr/bin/env python3
import json, os, pathlib, shutil, sys, time, subprocess
args=sys.argv[1:]
if 'FIXTURE_EXPECT_PROXY' in os.environ:
    expected=os.environ['FIXTURE_EXPECT_PROXY']
    assert all(os.environ.get(name,'')==expected for name in ('http_proxy','https_proxy','HTTP_PROXY','HTTPS_PROXY'))
    assert '--proxy' not in args
    assert not expected or all(expected not in argument for argument in args)
    if expected:
        assert os.environ.get('no_proxy','')=='' and os.environ.get('NO_PROXY','')==''
assert '--no-playlist' in args and '--ignore-config' in args
assert args[-1].startswith('https://www.youtube.com/watch?v=') and '&' not in args[-1]
output=args[args.index('-o')+1]
target=pathlib.Path(output.replace('%(ext)s','mp3'))
target.parent.mkdir(parents=True,exist_ok=True)
video=args[-1].split('v=')[-1]
mode_file=pathlib.Path(os.environ['FIXTURE_CONTROL'])/('mode-'+video)
mode=mode_file.read_text() if mode_file.exists() else ''
if mode=='child':
    child=subprocess.Popen([sys.executable,'-c',"import pathlib,time,sys; p=pathlib.Path(sys.argv[1]);\nwhile True:\n p.write_text(str(time.time_ns())); time.sleep(.02)",str(mode_file)+'.heartbeat'])
    pathlib.Path(str(mode_file)+'.childpid').write_text(str(child.pid))
    mode='wait'
while mode=='wait' and mode_file.exists():
    (target.parent/'partial.download').write_bytes(b'incomplete audio')
    time.sleep(.02)
if mode=='fail':
    (target.parent/'partial.download').write_bytes(b'incomplete audio')
    print('provider secret password https://secret.example/credential',file=sys.stderr)
    sys.exit(1)
if mode=='botchallenge':
    print("ERROR: Sign in to confirm you're not a bot. http://operator:proxy-password@invalid.example/credential",file=sys.stderr)
    sys.exit(1)
if mode=='grow':
    with (target.parent/'partial.download').open('wb') as working:
        while True:
            working.write(b'x'*1024)
            working.flush()
            time.sleep(.002)
shutil.copyfile(os.environ['FIXTURE_MP3'],target)
if mode=='corrupt':
    target.write_bytes(b'this is not playable MP3 audio')
if mode=='partial':
    target.write_bytes(target.read_bytes()[:2255])
target.with_suffix('.info.json').write_text(json.dumps({'title':'An <old> video & audio','uploader':'Fixture Creator','duration':0 if mode=='unknown' else (31 if mode=='wrongduration' else 1),'is_live':mode=='live','availability':'private' if mode=='private' else 'public','upload_date':'20100101'}))
