import json
import sqlite3
import sys
import tempfile
import unittest
from contextlib import redirect_stdout
from io import StringIO
from unittest.mock import patch
from pathlib import Path

import continuity as memory


class ContinuityTests(unittest.TestCase):
    def test_native_message_handoff_is_idempotent_and_original_immutable(self):
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            project = root / 'project';project.mkdir()
            (project / '需求逐项总清单.json').write_text('{"requirements":[]}', 'utf8')
            source = root / '输入/workbuddy/message.json';source.parent.mkdir(parents=True)
            value = {'id': 'fixture1', 'at': '2026-10-26T16:00:00+08:00', 'role': 'user', 'text': '部署前备份', 'attachments': [{'path': 'D:/fixture.png'}]}
            source.write_text(json.dumps(value, ensure_ascii=False), 'utf8')
            with patch.object(memory, 'MEMORY_HOME', root / '游园数据记忆库'), patch.object(memory, 'PRIVATE', root / 'private'), patch.object(memory, 'credential_setup', return_value=[]), redirect_stdout(StringIO()):
                memory.record(project, source, 'workbuddy')
                memory.record(project, source, 'workbuddy')
                db = sqlite3.connect(root / '游园数据记忆库/检索/knowledge.sqlite3')
                self.assertEqual(db.execute('SELECT count(*) FROM messages').fetchone()[0], 1)
                self.assertEqual(db.execute('SELECT count(*) FROM assets').fetchone()[0], 1)
                value['text'] = '不能覆盖原话';source.write_text(json.dumps(value), 'utf8')
                with self.assertRaises(ValueError):memory.record(project, source, 'workbuddy')
                self.assertEqual(db.execute('SELECT text FROM messages').fetchone()[0], '部署前备份')
                db.close()

    def test_native_handoff_cannot_read_a_private_client_directory(self):
        with tempfile.TemporaryDirectory() as temp:
            source = Path(temp) / 'private-client-message.json';source.write_text('{}')
            with self.assertRaises(ValueError):memory.record(Path(temp), source, 'workbuddy')

    def test_project_boundary_and_windows_slashes(self):
        self.assertTrue(memory.scope_ok('C:/Repo/apk/project', 'c:\\repo\\apk'))
        self.assertFalse(memory.scope_ok('C:/Repo/apk-other', 'c:\\repo\\apk'))

    def test_redaction_keeps_requirement_but_not_secret(self):
        text = '部署前备份；密码 dummy123//；Bearer abcd1234abcd1234abcd1234'
        result = memory.redact(text, ['dummy123//'])
        self.assertIn('部署前备份', result)
        self.assertNotIn('dummy123//', result)
        self.assertNotIn('abcd1234abcd1234abcd1234', result)

    def test_chunks_cover_the_entire_original(self):
        text = '保留模块原频率和全部功能，部署前备份。' * 100
        covered = set()
        for start, chunk in memory.split_text(text):
            self.assertEqual(text[start:start + len(chunk)], chunk)
            covered.update(range(start, start + len(chunk)))
        self.assertEqual(covered, set(range(len(text))))

    @unittest.skipUnless(sys.platform == 'win32', 'Windows DPAPI')
    def test_private_payload_roundtrip(self):
        raw = '私有原文与凭据，只用于本机授权调用'.encode('utf8')
        sealed = memory.protected(raw)
        self.assertNotIn(raw, sealed)
        self.assertEqual(memory.protected(sealed, True), raw)

    def test_actual_new_user_requests_and_source_locations_are_present(self):
        db = sqlite3.connect(memory.MEMORY_HOME / '检索/knowledge.sqlite3')
        texts = '\n'.join(x[0] for x in db.execute("SELECT text FROM messages WHERE role='user'"))
        for quote in ('记得部署前要备份', '不要着急断对话', '游园数据记忆库'):
            self.assertIn(quote, texts)
        self.assertEqual(db.execute('SELECT count(*) FROM messages WHERE id NOT IN (SELECT message_id FROM origins)').fetchone()[0], 0)
        self.assertEqual(db.execute("PRAGMA integrity_check").fetchone()[0], 'ok')

    def test_credentials_not_present_as_plaintext_in_retrieval(self):
        source = Path('D:/YouYuan-APK/private/linux-access.json')
        if not source.exists():
            self.skipTest('authorized source not available')
        data = json.loads(source.read_text('utf8'))
        password = str(data['password']).encode('utf8')
        self.assertTrue(bool(password))
        for file in (memory.MEMORY_HOME / '检索').glob('knowledge.sqlite3*'):
            self.assertFalse(password in file.read_bytes(), 'credential leaked into retrieval files')
        vault = sqlite3.connect(memory.PRIVATE / 'private.sqlite3')
        value = vault.execute("SELECT value_dpapi FROM credentials WHERE name='linux.password'").fetchone()
        self.assertTrue(value is not None)
        self.assertTrue(memory.protected(value[0], True) == password, 'credential roundtrip failed')


if __name__ == '__main__':
    unittest.main()
