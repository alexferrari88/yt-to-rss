#!/usr/bin/env python3
"""Check an already-built package through CLI/HTTP, replacement, backup and restore.

Run docker compose build first. Uses isolated Compose state and no host ports.
"""
import hashlib,json,pathlib,shutil,subprocess,tempfile
repo=pathlib.Path(__file__).resolve().parent.parent
tmp=pathlib.Path(tempfile.mkdtemp(prefix='2pod-package-'))
project='2pod-check-'+tmp.name.removeprefix('2pod-package-').lower()
override=tmp/'compose.test.yaml'
override.write_text(f"""services:
  2pod:
    environment:
      TWOPOD_BASE_URL: http://127.0.0.1:8080
      TWOPOD_FEED_LINK: http://127.0.0.1:8080
      TWOPOD_READ_TOKEN: ''
      TWOPOD_TELEGRAM_BOT_TOKEN: ''
      TWOPOD_TELEGRAM_OPERATOR_ID: ''
      TWOPOD_EXTRACTOR: /fixtures/extractor.py
      TWOPOD_POLL_INTERVAL: 20ms
      TWOPOD_MIN_FREE_BYTES: '0'
      TWOPOD_STORAGE_LIMIT_BYTES: '10737418240'
      TWOPOD_RETENTION: 720h
      TWOPOD_PROCESS_TIMEOUT: 1m
      FIXTURE_MP3: /fixtures/tone.mp3
      FIXTURE_CONTROL: /data
    volumes:
      - {repo}/acceptance/testdata:/fixtures:ro
""")
base=['docker','compose','-p',project,'-f',str(repo/'compose.yaml'),'-f',str(override)]
def run(args, text=None):
 r=subprocess.run(base+args,input=text,text=True,capture_output=True)
 if r.returncode: raise RuntimeError(f'Compose command failed {args[:3]}: {r.stderr[:2000]}')
 return r.stdout
check="""import hashlib,json,pathlib,subprocess,time,urllib.request,urllib.error,xml.etree.ElementTree as ET

def cli(*a):
 r=subprocess.run(['2pod',*a],text=True,capture_output=True)
 assert r.returncode==0, 'CLI failed '+r.stderr
 return r.stdout
for _ in range(100):
 try: cli('list');break
 except AssertionError: time.sleep(.05)
else: raise AssertionError('service did not become ready')
url=cli('feed-url').strip()
if PHASE=='first':
 s=json.loads(cli('add','https://youtu.be/abcdefghijk?t=81&list=ignored'))
 assert s['ID']=='abcdefghijk'
for _ in range(100):
 s=json.loads(cli('status','abcdefghijk'))
 if s['State']=='published':break
 time.sleep(.05)
else: raise AssertionError('fixture was not published: '+repr(s))
assert s['Title']=='An <old> video & audio'
assert s['Uploader']=='Fixture Creator'
rss=ET.fromstring(urllib.request.urlopen(url).read())
items=rss.findall('./channel/item');assert len(items)==1
assert items[0].findtext('guid')=='youtube:abcdefghijk'
e=items[0].find('enclosure');fixture=pathlib.Path('/fixtures/tone.mp3').read_bytes()
assert e.attrib['type']=='audio/mpeg'
assert int(e.attrib['length'])==len(fixture)
media=e.attrib['url'];resp=urllib.request.urlopen(media)
assert resp.headers['Content-Type']=='audio/mpeg'
assert resp.read()==fixture
head=urllib.request.urlopen(urllib.request.Request(media,method='HEAD'))
assert int(head.headers['Content-Length'])==len(fixture)
part=urllib.request.urlopen(urllib.request.Request(media,headers={'Range':'bytes=10-19'}))
assert part.status==206 and part.read()==fixture[10:20]
assert part.headers['Content-Range']=='bytes 10-19/'+str(len(fixture))
try: urllib.request.urlopen('http://127.0.0.1:8080/wrong/feed.xml')
except urllib.error.HTTPError as e: assert e.code==404
else: raise AssertionError('wrong read credential accepted')
print(json.dumps({'phase':PHASE,'id':s['ID'],'state':s['State'],'guid':items[0].findtext('guid'),'media_bytes':len(fixture),'media_sha256':hashlib.sha256(fixture).hexdigest(),'read_url_fingerprint':hashlib.sha256(url.encode()).hexdigest()}))
"""
try:
 run(['config','--quiet'])
 run(['up','-d','--no-build'])
 cid=run(['ps','-q','2pod']).strip()
 portmap=json.loads(subprocess.check_output(['docker','inspect','--format','{{json .HostConfig.PortBindings}}',cid],text=True))
 assert not portmap, 'unexpected host port binding'
 first=json.loads(run(['exec','-T','2pod','python3','-c',"PHASE='first'\n"+check]))
 run(['up','-d','--no-build','--force-recreate'])
 assert run(['ps','-q','2pod']).strip()!=cid,'container not replaced'
 second=json.loads(run(['exec','-T','2pod','python3','-c',"PHASE='replaced'\n"+check]))
 assert first['read_url_fingerprint']==second['read_url_fingerprint'],'read token changed'
 assert first['media_sha256']==second['media_sha256'],'media changed'
 run(['stop'])
 backup=subprocess.run(base+['run','--rm','--no-deps','-T','--entrypoint','tar','2pod','--exclude=control.sock','-C','/data','-czf','-','.'],capture_output=True)
 assert backup.returncode==0,'backup failed'
 archive=tmp/'state.tar.gz';archive.write_bytes(backup.stdout);archive.chmod(0o600)
 run(['down','--volumes','--remove-orphans'])
 restore=subprocess.run(base+['run','--rm','--no-deps','-T','--entrypoint','tar','2pod','-C','/data','-xzf','-'],input=backup.stdout,capture_output=True)
 assert restore.returncode==0,'restore failed'
 run(['up','-d','--no-build'])
 restored=json.loads(run(['exec','-T','2pod','python3','-c',"PHASE='restored'\n"+check]))
 assert restored['read_url_fingerprint']==first['read_url_fingerprint'],'restore changed read access'
 assert restored['media_sha256']==first['media_sha256'],'restore changed media'
 print(json.dumps({'verified':True,'docker_security_options':json.loads(subprocess.check_output(['docker','info','--format','{{json .SecurityOptions}}'],text=True)),'no_host_ports':True,'container_replacement':True,'backup_restore':True,'before':first,'after':second,'restored':restored},indent=2))
finally:
 try:
  run(['down','--volumes','--remove-orphans'])
 finally:
  shutil.rmtree(tmp)
