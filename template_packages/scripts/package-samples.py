from __future__ import annotations

import argparse
import json
import os
from pathlib import Path
import zipfile


FIXED_ZIP_TIME = (2026, 1, 1, 0, 0, 0)


def main() -> int:
    parser = argparse.ArgumentParser(description="Package AuraLogic template samples into ZIP artifacts.")
    parser.add_argument("--only", action="append", default=[], help="Only package the given sample directory name.")
    args = parser.parse_args()

    root = Path(__file__).resolve().parent.parent
    selected = {item.strip() for item in args.only if item and item.strip()}
    samples = discover_samples(root, selected)
    if not samples:
        raise SystemExit("no template package samples found")

    for sample_root in samples:
        package_sample(sample_root)
    print(f"packaged {len(samples)} template sample(s)")
    return 0


def discover_samples(root: Path, selected: set[str]) -> list[Path]:
    out: list[Path] = []
    for entry in sorted(root.iterdir(), key=lambda item: item.name.lower()):
        if not entry.is_dir():
            continue
        if entry.name == "scripts":
            continue
        if selected and entry.name not in selected:
            continue
        manifest_path = entry / "manifest.json"
        if manifest_path.is_file():
            out.append(entry)
    return out


def package_sample(sample_root: Path) -> None:
    manifest_path = sample_root / "manifest.json"
    manifest = json.loads(manifest_path.read_text(encoding="utf-8"))
    sample_name = str(manifest.get("name") or sample_root.name).strip() or sample_root.name
    zip_path = sample_root / f"{sample_root.name}.zip"
    file_paths = collect_files(sample_root)
    if not file_paths:
        raise RuntimeError(f"template sample {sample_root.name} is empty")

    if zip_path.exists():
        zip_path.unlink()

    with zipfile.ZipFile(zip_path, "w", compression=zipfile.ZIP_DEFLATED, compresslevel=9) as archive:
        for file_path in file_paths:
            arcname = file_path.relative_to(sample_root).as_posix()
            info = zipfile.ZipInfo(arcname, FIXED_ZIP_TIME)
            info.compress_type = zipfile.ZIP_DEFLATED
            info.create_system = 3
            info.external_attr = 0o644 << 16
            archive.writestr(info, file_path.read_bytes(), compress_type=zipfile.ZIP_DEFLATED, compresslevel=9)

    print(f"packaged {sample_root.name} ({sample_name}) -> {zip_path.name}")


def collect_files(sample_root: Path) -> list[Path]:
    out: list[Path] = []
    for file_path in sample_root.rglob("*"):
        if not file_path.is_file():
            continue
        if file_path.name.endswith(".zip"):
            continue
        out.append(file_path)
    return sorted(out, key=lambda item: item.relative_to(sample_root).as_posix())


if __name__ == "__main__":
    raise SystemExit(main())
