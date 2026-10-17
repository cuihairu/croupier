#!/usr/bin/env python3
"""Package implemented Agent binaries with config, role guide and checksums."""

import argparse
import hashlib
import io
import json
from pathlib import Path
import re
import tarfile
import zipfile


AGENTS = {
    "sidecar": ("croupier-agent", "agent.yaml", "agent.yaml"),
    "devops": ("devops-agent", "devops-agent.example.yaml", "devops-agent.yaml"),
    "capture": ("capture-agent", "capture-agent.example.yaml", "capture-agent.yaml"),
}


def document_bytes(source, root):
    """Keep links usable when documentation moves to the archive root."""
    def online_link(match):
        target = match.group(1)
        if "://" in target or target.startswith(("#", "/", "mailto:")):
            return match.group(0)
        path, separator, anchor = target.partition("#")
        resolved = (source.parent / path).resolve()
        if not resolved.suffix and resolved.with_suffix(".md").is_file():
            resolved = resolved.with_suffix(".md")
        relative = resolved.relative_to(root.resolve()).as_posix()
        return "](https://github.com/cuihairu/croupier/blob/main/" + relative + separator + anchor + ")"

    return re.sub(r"\]\(([^)]+)\)", online_link, source.read_text(encoding="utf-8")).encode("utf-8")


def package_agents(root, binaries, output, goos, goarch, roles):
    root, binaries, output = Path(root), Path(binaries), Path(output)
    # Validate the complete input set before writing any release assets.
    inputs = []
    for role in roles:
        name, source_config, target_config = AGENTS[role]
        if goarch in ("arm", "armv7") and role == "capture":
            raise ValueError("capture-agent requires a 64-bit target (CDC parser dependency)")
        binary = binaries / (name + (".exe" if goos == "windows" else ""))
        files = [
            (binary, binary.name),
            (root / "configs" / source_config, "configs/" + target_config),
            (root / "docs/operations/agent-comparison.md", "AGENTS.md"),
            (root / "docs/operations/agent-install.md", "INSTALL.md"),
        ]
        for source, _ in files:
            if not source.is_file() or source.stat().st_size == 0:
                raise ValueError(f"missing or empty Agent package input: {source}")
        inputs.append((role, name, binary, files))

    output.mkdir(parents=True, exist_ok=True)
    manifest = {"os": goos, "arch": goarch, "agents": []}
    checksums = []
    for role, name, binary, files in inputs:
        extension = ".zip" if goos == "windows" else ".tar.gz"
        archive = output / f"{name}-{goos}-{goarch}{extension}"
        if goos == "windows":
            with zipfile.ZipFile(archive, "w", zipfile.ZIP_DEFLATED) as bundle:
                for source, destination in files:
                    if source.suffix == ".md":
                        bundle.writestr(destination, document_bytes(source, root))
                    else:
                        bundle.write(source, destination)
        else:
            with tarfile.open(archive, "w:gz") as bundle:
                for source, destination in files:
                    if source.suffix == ".md":
                        content = document_bytes(source, root)
                        info = tarfile.TarInfo(destination)
                        info.size = len(content)
                        info.mode = 0o644
                        bundle.addfile(info, io.BytesIO(content))
                    else:
                        bundle.add(source, arcname=destination)
        with archive.open("rb") as stream:
            digest = hashlib.file_digest(stream, "sha256").hexdigest()
        with binary.open("rb") as stream:
            binary_digest = hashlib.file_digest(stream, "sha256").hexdigest()
        manifest["agents"].append(
            {"role": role, "binary": binary.name, "archive": archive.name,
             "sha256": digest, "binarySha256": binary_digest}
        )
        checksums.append(f"{digest}  {archive.name}\n")

    prefix = f"agents-{goos}-{goarch}"
    (output / (prefix + ".json")).write_text(
        json.dumps(manifest, ensure_ascii=False, indent=2) + "\n", encoding="utf-8"
    )
    (output / (prefix + ".sha256")).write_text("".join(checksums), encoding="utf-8")
    return manifest


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--root", default=".")
    parser.add_argument("--bin-dir", default="bin")
    parser.add_argument("--output", default="release")
    parser.add_argument("--os", required=True, choices=("linux", "darwin", "windows"))
    parser.add_argument("--arch", required=True, choices=("amd64", "arm64", "arm", "armv7"))
    parser.add_argument("--roles", nargs="+", choices=tuple(AGENTS), default=list(AGENTS))
    args = parser.parse_args()
    package_agents(args.root, args.bin_dir, args.output, args.os, args.arch, args.roles)


if __name__ == "__main__":
    main()
