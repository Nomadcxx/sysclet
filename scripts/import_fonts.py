#!/usr/bin/env python3
"""Rebuild sysclet's imported font packs.

    python3 scripts/import_fonts.py phosphor PHOSPHOR/assets/fonts/tdf
    python3 scripts/import_fonts.py bit SYSC-GO/assets/fonts

phosphor: copies TheDraw records whose names sysclet lacks into tdf-phosphor-NN.tdf.gz
(sourced from Phosphor commit 2cfb1cc991fb9e4ce5d97e8de239a368e804c4b5).
bit: converts sysc-Go .bit fonts to TheDraw block records in tdf-bit-NN.tdf.gz and
rewrites their section of THIRD-PARTY-NOTICES.md.

Each run replaces only its own packs and catalog entries. A name already in the
catalog keeps its existing record; the first record with a new name wins.
"""
import collections, glob, gzip, json, os, re, struct, sys

REPO = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
FONTS = os.path.join(REPO, "assets", "fonts")
CATALOG = os.path.join(FONTS, "catalog.json")
NOTICES = os.path.join(REPO, "THIRD-PARTY-NOTICES.md")
HDR = b"\x13TheDraw FONTS file\x1a"
TAG = b"\x55\xaa\x00\xff"
# Matches the existing packs and stays under ParseTDF's expanded glyph limit per pack.
CHUNK_BYTES = 600_000
# TDF records hold at most 16 name bytes.
SHORT_NAMES = {"entercommanditalic": "entercmditalic", "genericmobilesystem": "genericmobilesys", "pixelatedelegance": "pixelelegance"}


def tdf_records(data):
    """Yield (name, raw record) for each record; trailing data such as a SAUCE footer is ignored."""
    if not data.startswith(HDR):
        return
    off = len(HDR)
    while off + 213 <= len(data) and data[off:off + 4] == TAG:
        size = 213 + struct.unpack_from("<H", data, off + 23)[0]
        name = data[off + 5:off + 5 + data[off + 4]].rstrip(b"\0").decode("utf-8", "replace")
        yield name, data[off:off + size]
        off += size


def import_phosphor(src, have):
    picked = []
    for path in sorted(glob.glob(os.path.join(src, "*.tdf")), key=str.lower):
        for name, raw in tdf_records(open(path, "rb").read()):
            if name.lower() not in have and name.isprintable() and "�" not in name:
                have.add(name.lower())
                picked.append((name, raw))
    return picked, {}


def import_bit(src, have):
    cp437 = [int(x, 16) for x in re.findall(r"0x([0-9a-f]{4})", open(os.path.join(REPO, "cp437.go")).read())][:256]
    to_byte = {chr(v): i for i, v in enumerate(cp437) if i not in (0, 13)}
    to_byte[" "] = 32
    picked, seen_glyphs, meta = [], set(), {}
    for path in sorted(glob.glob(os.path.join(src, "*.bit"))):
        font = json.load(open(path))
        name, figlet = SHORT_NAMES.get(font["name"], font["name"]), "FIGlet" in font["license"]
        if name.lower() in have:
            continue
        if len(name.encode()) > 16 or not name.isascii() or not name.isprintable():
            print(f"skip {name}: name does not fit a TDF record")
            continue
        glyphs = {}
        for code in range(33, 127):
            rows = font["characters"].get(chr(code)) or []
            if figlet:  # FIGlet hardblanks and endmarks render as spaces.
                rows = [r.replace("$", " ") for r in rows]
            if any(r.strip() for r in rows):
                glyphs[code] = rows
        # Like sysc-Go's renderer, drop rows that are blank in every glyph.
        top = min((next(i for i, r in enumerate(g) if r.strip()) for g in glyphs.values()), default=0)
        bottom = min((next(i for i, r in enumerate(reversed(g)) if r.strip()) for g in glyphs.values()), default=0)
        offsets, block = [], b""
        try:
            for code in range(33, 127):
                rows = glyphs.get(code)
                if not rows:
                    offsets.append(0xFFFF)
                    continue
                rows = rows[top:len(rows) - bottom]
                glyph = bytes([min(max(map(len, rows)), 255), min(len(rows), 255)])
                glyph += b"".join(bytes(to_byte[c] for c in r) + b"\r" for r in rows) + b"\0"
                offsets.append(len(block))
                block += glyph
        except KeyError as e:
            print(f"skip {name}: character {e.args[0]!r} has no CP437 byte")
            continue
        if len(block) >= 0xFFFF or block in seen_glyphs:
            print(f"skip {name}: glyph data too large or same as an earlier font")
            continue
        seen_glyphs.add(block)
        have.add(name.lower())
        # BIT's editor adds one column between pixel-font glyphs; FIGlet glyphs carry their own spacing.
        spacing = 1 if figlet else 2
        raw = TAG + bytes([len(name.encode())]) + name.encode().ljust(16, b"\0") + bytes([1, spacing])
        raw += struct.pack("<H", len(block)) + struct.pack("<94H", *offsets) + block
        picked.append((name, raw))
        meta[name] = font
    return picked, meta


def write_bit_notices(meta):
    by_license = collections.defaultdict(list)
    for name, font in meta.items():
        by_license[font["license"]].append((name, font))
    out = [f"## sysc-Go BIT fonts\n\n`assets/fonts/tdf-bit-*.tdf.gz` contains {len(meta)} fonts converted from "
           "[sysc-Go](https://github.com/Nomadcxx/sysc-Go)'s `.bit` files into TheDraw block records by "
           "`scripts/import_fonts.py`. The conversion keeps each glyph's characters and maps FIGlet hardblanks (`$`) "
           "to spaces. Each font keeps the author and license its `.bit` file declares, listed below. A name in "
           "parentheses is the original `.bit` name where sysclet shortened it to fit a TDF record.\n"]
    for lic in sorted(by_license, key=lambda l: (-len(by_license[l]), l)):
        out.append(f"### {lic}\n")
        out += [f"- {n}{' (' + f['name'] + ')' if f['name'] != n else ''}: {f['author']}"
                for n, f in sorted(by_license[lic], key=lambda x: x[0].lower())]
        out.append("")
    text = open(NOTICES).read()
    start = text.find("\n## sysc-Go BIT fonts")
    open(NOTICES, "w").write((text if start < 0 else text[:start]).rstrip("\n") + "\n\n" + "\n".join(out))


def main():
    if len(sys.argv) != 3 or sys.argv[1] not in ("phosphor", "bit"):
        sys.exit(__doc__)
    kind, src = sys.argv[1], sys.argv[2]
    prefix = f"tdf-{kind}-"
    catalog = [e for e in json.load(open(CATALOG)) if not e["pack"].startswith(prefix)]
    for old in glob.glob(os.path.join(FONTS, prefix + "*.tdf.gz")):
        os.remove(old)
    have = {e["name"].lower() for e in catalog}
    picked, meta = (import_phosphor if kind == "phosphor" else import_bit)(src, have)
    if not picked:
        sys.exit(f"no new fonts found in {src}")

    chunks = [[]]
    for item in picked:
        if chunks[-1] and sum(len(r) for _, r in chunks[-1]) + len(item[1]) > CHUNK_BYTES:
            chunks.append([])
        chunks[-1].append(item)
    for c, chunk in enumerate(chunks):
        pack = f"{prefix}{c:02d}.tdf"
        with gzip.GzipFile(os.path.join(FONTS, pack + ".gz"), "wb", mtime=0) as f:
            f.write(HDR + b"".join(raw for _, raw in chunk))
        catalog += [{"name": name, "pack": pack, "record": i} for i, (name, _) in enumerate(chunk)]
    catalog.sort(key=lambda e: (e["name"].lower(), e["name"]))
    open(CATALOG, "w").write(json.dumps(catalog, separators=(",", ":")))
    if kind == "bit":
        write_bit_notices(meta)
    print(f"{kind}: {len(picked)} fonts in {len(chunks)} packs; catalog has {len(catalog)}")


if __name__ == "__main__":
    main()
