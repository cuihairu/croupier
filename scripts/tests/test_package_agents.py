import hashlib
import importlib.util
import json
from pathlib import Path
import tarfile
import tempfile
import unittest
import zipfile


ROOT = Path(__file__).resolve().parents[2]
SPEC = importlib.util.spec_from_file_location("package_agents", ROOT / "scripts/package-agents.py")
PACKAGER = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(PACKAGER)


class AgentPackageTests(unittest.TestCase):
    def test_archive_contents_and_checksums(self):
        for goos in ("linux", "windows"):
            with self.subTest(goos=goos), tempfile.TemporaryDirectory() as temporary:
                folder = Path(temporary)
                binaries, output = folder / "bin", folder / "release"
                binaries.mkdir()
                for name, _, _ in PACKAGER.AGENTS.values():
                    (binaries / (name + (".exe" if goos == "windows" else ""))).write_bytes(b"binary")
                manifest = PACKAGER.package_agents(ROOT, binaries, output, goos, "amd64", list(PACKAGER.AGENTS))
                self.assertEqual({a["role"] for a in manifest["agents"]}, {"sidecar", "devops", "capture"})
                self.assertEqual(json.loads((output / f"agents-{goos}-amd64.json").read_text()), manifest)
                checksums = (output / f"agents-{goos}-amd64.sha256").read_text()
                for agent in manifest["agents"]:
                    archive = output / agent["archive"]
                    digest = hashlib.sha256(archive.read_bytes()).hexdigest()
                    self.assertEqual(digest, agent["sha256"])
                    self.assertIn(f"{digest}  {archive.name}", checksums)
                    if goos == "windows":
                        with zipfile.ZipFile(archive) as bundle:
                            names = bundle.namelist()
                            guide = bundle.read("AGENTS.md").decode("utf-8")
                    else:
                        with tarfile.open(archive) as bundle:
                            names = bundle.getnames()
                            guide = bundle.extractfile("AGENTS.md").read().decode("utf-8")
                    self.assertIn(agent["binary"], names)
                    self.assertIn("AGENTS.md", names)
                    self.assertIn("INSTALL.md", names)
                    expected_config = PACKAGER.AGENTS[agent["role"]][2]
                    self.assertIn("configs/" + expected_config, names)
                    self.assertIn("https://github.com/cuihairu/croupier/blob/main/docs/", guide)
                    self.assertNotIn("synthetics-agent", names)

    def test_missing_binary_prevents_partial_release(self):
        with tempfile.TemporaryDirectory() as temporary:
            folder = Path(temporary)
            (folder / "croupier-agent").write_bytes(b"binary")
            with self.assertRaisesRegex(ValueError, "missing or empty"):
                PACKAGER.package_agents(ROOT, folder, folder / "out", "linux", "amd64", list(PACKAGER.AGENTS))
            self.assertFalse((folder / "out").exists())

    def test_armv7_package_has_only_supported_roles(self):
        with tempfile.TemporaryDirectory() as temporary:
            folder = Path(temporary)
            for name in ("croupier-agent", "devops-agent"):
                (folder / name).write_bytes(b"binary")
            manifest = PACKAGER.package_agents(ROOT, folder, folder / "out", "linux", "arm", ["sidecar", "devops"])
            self.assertEqual(len(manifest["agents"]), 2)
            with self.assertRaisesRegex(ValueError, "64-bit"):
                PACKAGER.package_agents(ROOT, folder, folder / "invalid", "linux", "arm", ["capture"])


if __name__ == "__main__":
    unittest.main()
