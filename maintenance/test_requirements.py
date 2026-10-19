import json,tempfile,unittest
from pathlib import Path
from requirements import close,render,load

class LedgerTests(unittest.TestCase):
 def setUp(self):
  self.temp=tempfile.TemporaryDirectory();self.root=Path(self.temp.name);self.project=self.root/'project';self.project.mkdir();(self.project/'AGENTS.md').write_text('fixture');(self.root/'outputs').mkdir()
  self.path=self.project/'需求逐项总清单.json';self.path.write_text(json.dumps({'all_complete':False,'requirements':[{'id':'A01','module':'fixture','priority':'P0','request':'Keep original requirement','status':'未完成'},{'id':'A02','module':'fixture','priority':'P1','request':'Keep pending item','status':'未完成'}]}),encoding='utf8')
 def tearDown(self):self.temp.cleanup()
 def test_missing_or_outside_evidence_never_closes(self):
  before=self.path.read_bytes()
  for path in ['outputs/missing.json','../../outside.json']:
   with self.assertRaises(ValueError):close(self.project,'A01',[path],'端到端通过','claim')
   self.assertEqual(before,self.path.read_bytes())
 def test_close_clears_active_entry_without_erasing_history(self):
  (self.root/'outputs/proof.json').write_text('{"passed":true}')
  result=close(self.project,'A01',['outputs/proof.json'],'端到端通过','fixture acceptance')
  self.assertEqual(result['pending'],1);self.assertEqual(result['closed'],1)
  self.assertNotIn('Keep original requirement',(self.project/'项目待办.md').read_text('utf8'));self.assertIn('Keep original requirement',(self.project/'已完成与撤回归档.md').read_text('utf8'))
  self.assertEqual(len(load(self.project)[2]),2);self.assertFalse(json.loads(self.path.read_text('utf8'))['all_complete'])
 def test_duplicate_ids_fail_before_partial_render(self):
  data=json.loads(self.path.read_text('utf8'));data['requirements'][1]['id']='A01';self.path.write_text(json.dumps(data),encoding='utf8')
  with self.assertRaises(ValueError):render(self.project)
  self.assertFalse((self.project/'项目待办.md').exists())

if __name__=='__main__':unittest.main()
