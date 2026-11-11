"""Cache only unit 2 and its first contents page from the user supplied PDF.

Read-only for the source PDF. Uses pypdf + pypdfium2 + Pillow; no OCR or MIDI
transcription. Run with the bundled Codex Python runtime.
"""

from __future__ import annotations

import argparse
import hashlib
import json
import logging
import re
from pathlib import Path

import pypdfium2 as pdfium
from PIL import Image, ImageDraw, ImageFont
from pypdf import PdfReader


def digest(path: Path) -> str:
    with path.open("rb") as stream:
        return hashlib.file_digest(stream, "sha256").hexdigest()


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("source_pdf", type=Path)
    args = parser.parse_args()
    source = args.source_pdf.resolve(strict=True)
    root = Path(__file__).resolve().parents[1]
    target = root / "source"
    pages_dir = target / "pages"
    pages_dir.mkdir(parents=True, exist_ok=True)
    logging.getLogger("pypdf").setLevel(logging.ERROR)
    before = digest(source)
    reader = PdfReader(source)
    if len(reader.pages) != 127:
        raise ValueError("Source page count changed; recheck the page mapping")
    cover = reader.pages[19].extract_text() or ""
    boundary = reader.pages[35].extract_text() or ""
    if "旋律写作" not in cover or "第三单元" not in boundary:
        raise ValueError("Unit boundary does not match the verified PDF")

    selected = [2, *range(20, 36)]
    records = []
    clean_md = [
        "# 第二单元定向文本提取（源材料）",
        "",
        "仅 PDF 第2页目录及第20-35页；保留提取顺序，删除页脚乱码行。",
        "谱例音符和部分乐理符号不在文本层，须查看同页 PNG；本文件不是校订乐谱。",
        "",
    ]
    document = pdfium.PdfDocument(str(source))
    for number in selected:
        raw = reader.pages[number - 1].extract_text() or ""
        clean = "\n".join(
            line for line in raw.splitlines()
            if not re.search(r"[\x00-\x08\x0e-\x1f]", line)
        )
        book = number - 5 if 21 <= number <= 35 else None
        name = f"pdf-{number:03d}"
        name += f"_book-{book:03d}" if book is not None else "_contents" if number == 2 else "_unit-cover"
        image_path = pages_dir / f"{name}.png"
        page = document[number - 1]
        bitmap = page.render(scale=1.8 if number in {23, 26, 27, 34, 35} else 1.35)
        bitmap.to_pil().save(image_path, optimize=True)
        bitmap.close()
        page.close()
        records.append({
            "pdf_page": number,
            "printed_page": book,
            "kind": "contents" if number == 2 else "unit_cover" if number == 20 else "body",
            "raw_text": raw,
            "clean_text": clean,
            "image": image_path.relative_to(root).as_posix(),
            "image_sha256": digest(image_path),
        })
        label = f"PDF 第 {number} 页"
        if book is not None:
            label += f" / 书内第 {book} 页"
        clean_md += [f"## {label}", "", f"![原页]({image_path.relative_to(target).as_posix()})", "", clean, ""]
    document.close()
    (target / "pages.json").write_text(json.dumps(records, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
    (target / "unit2-extracted-text.md").write_text("\n".join(clean_md), encoding="utf-8")

    body_records = [row for row in records if row["pdf_page"] != 2]
    columns, cell_w, cell_h = 4, 300, 430
    sheet = Image.new("RGB", (columns * cell_w, 4 * cell_h), "white")
    draw = ImageDraw.Draw(sheet)
    font = ImageFont.load_default(size=16)
    for idx, row in enumerate(body_records):
        x, y = idx % columns * cell_w, idx // columns * cell_h
        with Image.open(root / row["image"]) as im:
            im = im.convert("RGB")
            im.thumbnail((cell_w - 10, cell_h - 34))
            sheet.paste(im, (x + (cell_w - im.width) // 2, y + 28))
        label = f"PDF {row['pdf_page']} / book {row['printed_page'] if row['printed_page'] else 'cover'}"
        draw.text((x + 10, y + 5), label, font=font, fill="black")
    contact = target / "unit2-contact-sheet.png"
    sheet.save(contact, optimize=True)
    after = digest(source)
    if before != after:
        raise RuntimeError("Source PDF changed during extraction")
    manifest = {
        "schema_version": "vit.lesson_source_cache.v0",
        "source_path_at_extraction": str(source),
        "source_filename": source.name,
        "source_sha256": before,
        "source_bytes": source.stat().st_size,
        "total_pdf_pages": len(reader.pages),
        "unit_title": "第二单元 旋律写作",
        "unit_cover_pdf_page": 20,
        "body_pdf_pages": [21, 35],
        "body_printed_pages": [16, 30],
        "context_pdf_pages": [2],
        "next_unit_pdf_page_checked": 36,
        "text_is_score_transcription": False,
        "notes": [
            "文件名包含2017年版；未从版权页独立核定出版版次。",
            "所有谱例以原页图像为准；文本提取不恢复音符。",
            "节奏或音符人工校订若未单独声明，不得认为已验证可播放。",
        ],
        "pages": [{k: row[k] for k in ("pdf_page", "printed_page", "kind", "image", "image_sha256")} for row in records],
        "source_unchanged": before == after,
        "contact_sheet": contact.relative_to(root).as_posix(),
    }
    (target / "manifest.json").write_text(json.dumps(manifest, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
    print(json.dumps({
        "source_unchanged": True,
        "source_sha256": before,
        "cached_pages": len(records),
        "unit_body_pages": 15,
        "cache_bytes": sum(f.stat().st_size for f in target.rglob("*") if f.is_file()),
        "root": str(root),
        "contact_sheet": str(contact),
    }, ensure_ascii=False))


if __name__ == "__main__":
    main()
