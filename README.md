# dwarf-check

DWARF checker. Looks for and reports insanities in DWARF info. This tool is designed primarily to locate bad abstract origin references. Example:

% ./dwarf-check suspect-executable
unresolved abstract origin ref from DIE at offset 0x83f9f7 to bad offset 0x83de0b
%

