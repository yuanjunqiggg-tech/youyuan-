"""Render one authoritative ledger without resetting requirements or deleting evidence."""
import argparse, json
from pathlib import Path

def load(project):
    project=Path(project).resolve();path=project/'需求逐项总清单.json'
    data=json.loads(path.read_text(encoding='utf-8-sig'));rows=data['requirements']
    ids=[r['id'] for r in rows]
    if len(ids)!=len(set(ids)):raise ValueError('需求编号重复，拒绝生成可能漏项的清单')
    if not (project/'AGENTS.md').is_file():raise ValueError('目标不是已核实的项目目录')
    return project,data,rows

def category(row):
    status=row.get('status','')
    if status.startswith('已撤回') or status.startswith('已撤销'):return 'withdrawn'
    if row.get('closed_at') and row.get('evidence') and status.startswith('已完成'):return 'closed'
    return 'pending'

def safe(value):return str(value or '').replace('|','\\|').replace('\n','<br>')

def table(rows):
    lines=['| 编号 / 优先级 | 要求 | 状态与证据级别 | 当前阶段 / 剩余条件 |','|---|---|---|---|']
    for r in rows:
        evidence='、'.join(r.get('evidence',[]))
        lines.append('| '+safe(r['id']+' / '+r.get('priority','P1'))+' | '+safe(r['request'])+' | '+safe(r.get('status','未完成')+'；'+r.get('verification','待验收'))+' | '+safe(r.get('stage',''))+'<br>剩余：'+safe(r.get('remaining',''))+('<br>证据：'+safe(evidence) if evidence else '')+' |')
    return '\n'.join(lines)

def render(project):
    project,data,rows=load(project)
    pending=[r for r in rows if category(r)=='pending'];closed=[r for r in rows if category(r)=='closed'];withdrawn=[r for r in rows if category(r)=='withdrawn']
    stamp=data.get('updated_at','未记录')
    header=f'更新时间：{stamp}。共 {len(rows)} 项；待办 {len(pending)} 项；已验收归档 {len(closed)} 项；已撤回 {len(withdrawn)} 项。\n\n'
    full='# 游园工具箱：原始需求逐项总清单\n\n'+header+'**没有全部完成。** 本表来自同一个JSON；旧要求和证据不删除。仅验收关闭的条目从活动待办移入归档，不以构建或阶段交付宣称整体完成。未取得的外部聊天/仓库正文不声称已经阅读。\n\n'
    for module in dict.fromkeys(r['module'] for r in rows):full+='## '+module+'\n\n'+table([r for r in rows if r['module']==module])+'\n\n'
    full+='活动清单见[项目待办](项目待办.md)，关闭记录见[已完成与撤回归档](已完成与撤回归档.md)。\n'
    (project/'需求逐项总清单-2026-10-17.md').write_text(full,encoding='utf-8')
    (project/'项目待办.md').write_text('# 游园工具箱：每次继续先读\n\n'+header+'完成并验收的任务已从此活动列表移除；原始要求及证据仍在总账和归档中。不能只凭状态文字关闭，需 closed_at 与 evidence。\n\n'+table(pending)+'\n',encoding='utf-8')
    (project/'已完成与撤回归档.md').write_text('# 已完成与撤回归档\n\n'+header+'## 已验收关闭\n\n'+table(closed)+'\n\n## 已撤回，不执行\n\n'+table(withdrawn)+'\n',encoding='utf-8')
    return {'total':len(rows),'pending':len(pending),'closed':len(closed),'withdrawn':len(withdrawn),'all_complete':not pending}

def close(project,identifier,evidence,level,note):
    project,data,rows=load(project)
    row=next((r for r in rows if r['id']==identifier),None)
    if row is None:raise ValueError('不存在的需求编号')
    if not note or level!='端到端通过':raise ValueError('关闭需完整验收说明和端到端通过级别；部分进度请继续记在原编号')
    for item in evidence:
        p=(project.parent/item).resolve()
        if not p.is_relative_to(project.parent) or not p.is_file() or not p.stat().st_size:raise ValueError('证据不存在、为空或超出项目范围：'+item)
    if not evidence:raise ValueError('缺少证据')
    from datetime import datetime
    stamp=datetime.now().astimezone().isoformat()
    row.update(status='已完成（验收归档）',closed_at=stamp,verification=level,evidence=evidence,stage=note,remaining='无；后续回归失败时重新打开原编号')
    data['updated_at']=stamp
    data['all_complete']=not any(category(r)=='pending' for r in rows)
    (project/'需求逐项总清单.json').write_text(json.dumps(data,ensure_ascii=False,indent=2),encoding='utf-8')
    return render(project)

if __name__=='__main__':
    p=argparse.ArgumentParser();p.add_argument('--project',required=True);p.add_argument('--render',action='store_true');p.add_argument('--close');p.add_argument('--evidence',action='append',default=[]);p.add_argument('--level',default='');p.add_argument('--note',default='');args=p.parse_args()
    if args.close:result=close(args.project,args.close,args.evidence,args.level,args.note)
    elif args.render:result=render(args.project)
    else:
        _,data,rows=load(args.project);result={'updated_at':data.get('updated_at'),'all_complete':data.get('all_complete'),'pending':[{'id':r['id'],'priority':r.get('priority'),'request':r['request'],'stage':r.get('stage'),'remaining':r.get('remaining')} for r in rows if category(r)=='pending']}
    print(json.dumps(result,ensure_ascii=False,indent=2))
