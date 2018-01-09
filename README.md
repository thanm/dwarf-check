# dwarf-check

DWARF checker. Looks for and reports insanities in DWARF info. This tool is designed primarily to locate bad abstract origin references. Example:

```
$ ./dwarf-check suspect-executable
unresolved abstract origin ref from DIE 270354 at offset 0x83f9f7 to bad offset 0x83de0b

0x83f9f7: FormalParameter
at=AbstractOrigin val=0x83de0b
at=Location val=0x9108

Parent:
0x83f9d3: Subprogram
at=AbstractOrigin val=0x83dda6
at=Lowpc val=0xc24ae0
at=Highpc val=0xc24aea
at=FrameBase val=0x9c
$
```
