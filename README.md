# dwarf-check

DWARF checker. Looks for and reports insanities in DWARF info. This tool is designed primarily to locate bad abstract origin references. Example:

```
$ ./dwarf-check suspect-executable
unresolved abstract origin ref from DIE 270354 at offset 0x83f9e7 to bad offset 0x83de0b

0x83f9e7: FormalParameter
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

In general if a compiler emits bad/illegal DWARF, the incorrect DWARF won't typically be caught by GDB unless you happen to doing something in your debug session that happens to cause GDB to use the specific bad portion of the DWARF (for example, printing variable Y in routine X). 

In contrast, when linking a program on the Mac, dsymutil has to post-process and fix up all of the DWARF for a module, so it tends to error/crash right away (as opposed to having latent DWARF bugs lurking). 

The intent of this tool was to have something easily buildable and runnable on Linux that would detect the same classes of problems that would cause dsymutil errors on the Mac.
