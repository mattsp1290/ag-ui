import json
import pytest
from ag_ui_claude_sdk.native_history import NativeHistory
SESSION = '11111111-1111-4111-8111-111111111111'
def fixture(tmp_path):
    folder = tmp_path / 'project' / SESSION / 'subagents'
    folder.mkdir(parents=True)
    def row(call, name, child=False):
        return dict(type='assistant', uuid=call+'-row', version='2.1.306', sessionId=SESSION,
                    **({'agentId':'abc123'} if child else {}),
                    message={'role':'assistant','content':[dict(type='tool_use',id=call,name=name,input={'file_path':'fixture'})]})
    (folder.parent.parent / (SESSION+'.jsonl')).write_text(json.dumps(row('parent','Agent'))+'\n')
    path=folder/'agent-abc123.jsonl';path.write_text(json.dumps(row('question','Read',True))+'\n')
    return path, NativeHistory(tmp_path,SESSION,['Read'])
def test_reconstruct_and_answer(tmp_path):
    path, history=fixture(tmp_path);before=path.read_bytes();checkpoint=history.inspect()
    assert checkpoint['pending'][0]['id']=='question'
    assert NativeHistory(tmp_path,SESSION,['Read']).inspect()==checkpoint
    assert path.read_bytes()==before
    history.answer(checkpoint, {'question':{'content':'ACTUAL RESULT','is_error':False}})
    assert NativeHistory(tmp_path,SESSION,['Read']).inspect()['pending']==[]
    with pytest.raises(ValueError):history.answer(checkpoint,{'question':{'content':'duplicate'}})
def test_ambiguous_children(tmp_path):
    path, history=fixture(tmp_path)
    path.with_name('agent-def456.jsonl').write_bytes(path.read_bytes())
    with pytest.raises(ValueError,match='one child'):history.inspect()
def test_path_traversal(tmp_path):
    with pytest.raises(ValueError,match='session'):NativeHistory(tmp_path,'../escape',['Read'])
def test_repair_only_after_child_completion(tmp_path):
    path, history=fixture(tmp_path);checkpoint=history.inspect()
    history.answer(checkpoint,{'question':{'content':'DENIED','is_error':True}})
    with pytest.raises(ValueError,match='not completed'):history.repair(checkpoint)
    with path.open('a') as f:f.write(json.dumps(dict(type='assistant',uuid='final-row',sessionId=SESSION,agentId='abc123',version='2.1.306',message={'role':'assistant','stop_reason':'end_turn','content':[{'type':'text','text':'Child respected the denial'}]}))+'\n')
    history.repair(checkpoint)
    assert history.inspect() is None
    assert 'Child respected the denial' in checkpoint['parentPath'].read_text()
    assert '"is_error": true' in path.read_text()
def test_empty_completed_response(tmp_path):
    path, history=fixture(tmp_path);checkpoint=history.inspect()
    history.answer(checkpoint,{'question':{'content':''}})
    with path.open('a') as f:f.write(json.dumps(dict(type='assistant',uuid='empty-final',version='2.1.306',message={'role':'assistant','stop_reason':'end_turn','content':[]}))+'\n')
    assert history.completed(checkpoint)
    history.repair(checkpoint)
    assert history.inspect() is None
