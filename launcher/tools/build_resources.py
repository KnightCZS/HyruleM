"""Generate the Windows icon, manifest and COFF resource object.
Development-only dependency: Pillow. The prebuilt .syso is included with the sources.
"""
from pathlib import Path
from io import BytesIO
import struct
from PIL import Image, ImageDraw

ROOT=Path(__file__).resolve().parents[1]
assets=ROOT/"assets";assets.mkdir(exist_ok=True)
image=Image.new("RGBA",(256,256),(13,19,26,255))
draw=ImageDraw.Draw(image)
draw.rounded_rectangle((18,18,238,238),radius=54,fill=(23,32,42),outline=(74,79,65),width=3)
draw.polygon([(128,40),(209,128),(128,216),(47,128)],outline=(218,188,115),width=10)
draw.line([(90,160),(128,86),(166,160)],fill=(233,236,231),width=11)
draw.line([(106,139),(150,139)],fill=(218,188,115),width=9)
png=BytesIO();image.save(png,format="PNG");icon=png.getvalue()
ico=struct.pack("<HHH",0,1,1)+struct.pack("<BBBBHHII",0,0,0,0,1,32,len(icon),22)+icon
(assets/"launcher.ico").write_bytes(ico)
manifest=b'''<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<assembly xmlns="urn:schemas-microsoft-com:asm.v1" manifestVersion="1.0">
<assemblyIdentity version="0.4.1.0" processorArchitecture="amd64" name="HyruleLauncher.Offline" type="win32"/>
<description>Hyrule Launcher</description>
<trustInfo xmlns="urn:schemas-microsoft-com:asm.v3"><security><requestedPrivileges><requestedExecutionLevel level="asInvoker" uiAccess="false"/></requestedPrivileges></security></trustInfo>
<compatibility xmlns="urn:schemas-microsoft-com:compatibility.v1"><application><supportedOS Id="{8e0f7a12-bfb3-4fe8-b9a5-48fd50a15a9a}"/></application></compatibility>
<application xmlns="urn:schemas-microsoft-com:asm.v3"><windowsSettings>
<dpiAware xmlns="http://schemas.microsoft.com/SMI/2005/WindowsSettings">true/pm</dpiAware>
<dpiAwareness xmlns="http://schemas.microsoft.com/SMI/2016/WindowsSettings">PerMonitorV2,PerMonitor</dpiAwareness>
<longPathAware xmlns="http://schemas.microsoft.com/SMI/2016/WindowsSettings">true</longPathAware>
</windowsSettings></application>
</assembly>
'''
(assets/"app.manifest").write_bytes(manifest)
group=struct.pack("<HHH",0,1,1)+struct.pack("<BBBBHHIH",0,0,0,0,1,32,len(icon),1)
resources=[(3,1,icon),(14,1,group),(24,1,manifest)]
n=len(resources)
root_size=16+8*n
types=[root_size+i*24 for i in range(n)]
langs=[root_size+24*n+i*24 for i in range(n)]
entries=[root_size+48*n+i*16 for i in range(n)]
raw=bytearray(root_size+48*n+16*n)
def directory(offset,count):struct.pack_into("<IIHHHH",raw,offset,0,0,0,0,0,count)
directory(0,n)
relocs=[]
for i,(kind,resource_id,data) in enumerate(resources):
 struct.pack_into("<II",raw,16+8*i,kind,0x80000000|types[i])
 directory(types[i],1);struct.pack_into("<II",raw,types[i]+16,resource_id,0x80000000|langs[i])
 directory(langs[i],1);struct.pack_into("<II",raw,langs[i]+16,1033,entries[i])
 raw+=b"\0"*((-len(raw))%4)
 offset=len(raw);raw+=data
 struct.pack_into("<IIII",raw,entries[i],offset,len(data),0,0)
 relocs.append(struct.pack("<IIH",entries[i],0,3))
relocation=b"".join(relocs)
symbol_offset=60+len(raw)+len(relocation)
header=struct.pack("<HHIIIHH",0x8664,1,0,symbol_offset,1,0,0)
section=struct.pack("<8sIIIIIIHHI",b".rsrc\0\0\0",0,0,len(raw),60,60+len(raw),0,n,0,0x40000040)
symbol=struct.pack("<8sIhHBB",b".rsrc\0\0\0",0,1,0,3,0)
out=ROOT/"cmd/hyrule/resources_windows_amd64.syso"
out.write_bytes(header+section+raw+relocation+symbol+struct.pack("<I",4))
print(out)

