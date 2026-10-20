#!/usr/bin/env python3
import os
from pathlib import Path
import subprocess
import tempfile
import unittest


WORKFLOW = Path(__file__).resolve().parent.parent / '.github/workflows/docker-build-and-push.yml'
SOURCE = WORKFLOW.read_text()
START = SOURCE.index('          refs=""')
END = SOURCE.index('\n      - name: Install Cosign', START)
SCRIPT = '\n'.join(line[10:] for line in SOURCE[START:END].splitlines())
REPOSITORY = 'https://github.com/example/obot.git'


class ManifestTests(unittest.TestCase):
    def run_manifest(self, tag):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            commands = root / 'commands'
            commands.mkdir()
            digests = root / 'digests'
            digests.mkdir()
            for digest in ['a' * 64, 'b' * 64]:
                (digests / digest).touch()
            docker = commands / 'docker'
            docker.write_text('#!/bin/bash\nprintf "%s\\n" "$*" >> "$DOCKER_CALLS"\n')
            docker.chmod(0o755)
            git = commands / 'git'
            git.write_text(
                '#!/bin/bash\n'
                '[[ "$*" == "ls-remote --tags --refs $EXPECTED_REPOSITORY v*" ]] || exit 19\n'
                'printf "abc\\trefs/tags/v0.25.6\\nabc\\trefs/tags/v0.26.3\\nabc\\trefs/tags/v0.27.0-rc1\\n"\n'
            )
            git.chmod(0o755)
            calls = root / 'calls'
            result = subprocess.run(
                ['bash', '-e', '-c', SCRIPT],
                cwd=digests,
                env={
                    **os.environ,
                    'PATH': f'{commands}:{os.environ["PATH"]}',
                    'TAG': tag,
                    'IMAGE': 'example.invalid/obot',
                    'ENTERPRISE_IMAGE': 'example.invalid/obot-enterprise',
                    'GITHUB_SERVER_URL': 'https://github.com',
                    'GITHUB_REPOSITORY': 'example/obot',
                    'EXPECTED_REPOSITORY': REPOSITORY,
                    'DOCKER_CALLS': str(calls),
                },
                capture_output=True,
                text=True,
                check=False,
            )
            self.assertEqual(result.returncode, 0, result.stderr)
            return calls.read_text().splitlines()

    def test_highest_stable_updates_both_latest_tags_without_checkout(self):
        calls = self.run_manifest('v0.26.3')
        self.assertEqual(len(calls), 4, calls)
        self.assertIn('-t example.invalid/obot:latest ', calls[2])
        self.assertIn('-t example.invalid/obot-enterprise:latest ', calls[3])
        for call in calls:
            self.assertIn('sha256:' + 'a' * 64, call)
            self.assertIn('sha256:' + 'b' * 64, call)

    def test_old_stable_does_not_move_latest(self):
        calls = self.run_manifest('v0.25.6')
        self.assertEqual(len(calls), 2, calls)
        self.assertTrue(all(':latest ' not in call for call in calls))

    def test_prerelease_does_not_move_latest(self):
        calls = self.run_manifest('v0.27.0-rc1')
        self.assertEqual(len(calls), 2, calls)
        self.assertTrue(all(':latest ' not in call for call in calls))

    def test_explicit_latest_creates_both_manifests(self):
        calls = self.run_manifest('latest')
        self.assertEqual(len(calls), 2, calls)
        self.assertIn('-t example.invalid/obot:latest ', calls[0])
        self.assertIn('-t example.invalid/obot-enterprise:latest ', calls[1])


if __name__ == '__main__':
    unittest.main()
