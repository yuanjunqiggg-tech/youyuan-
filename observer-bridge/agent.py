"""Feed actual client/proxy/engine event summaries, one JSON object per stdin line.
This program is the authenticated sender, not a game traffic interceptor.
"""
import argparse
import json
import os
from pathlib import Path
import sys
import time
import urllib.request

class Agent:
    def __init__(self, config_path):
        self.path = Path(config_path)
        self.config = json.loads(self.path.read_text(encoding='utf-8-sig'))
        self.sequence = int(self.config.get('sequence', 0))
    def submit(self, events):
        if not events:
            return
        next_sequence = self.sequence + 1
        # Save before send: a crash may skip a sequence, but cannot reuse it.
        self.config['sequence'] = next_sequence
        temporary = self.path.with_suffix(self.path.suffix + '.tmp')
        temporary.write_text(json.dumps(self.config, ensure_ascii=False), encoding='utf-8')
        os.chmod(temporary, 0o600)
        temporary.replace(self.path)
        self.sequence = next_sequence
        payload = {'source_id': self.config['source_id'], 'sequence': next_sequence, 'events': events}
        body = json.dumps(payload, ensure_ascii=False, separators=(',', ':')).encode()
        headers = {'Content-Type': 'application/json', 'X-Youyuan-Observer-Token': self.config['token']}
        headers.update(self.config.get('headers', {}))
        request = urllib.request.Request(self.config['url'], body, headers, method='POST')
        last_error = None
        for attempt in range(3):
            try:
                with urllib.request.urlopen(request, timeout=15) as response:
                    result = json.load(response)
                if not result.get('ok'):
                    raise RuntimeError(result.get('error', 'bridge rejected request'))
                return result['result']
            except Exception as error:
                last_error = error
                if attempt < 2:
                    time.sleep(.2 * (attempt + 1))
        raise RuntimeError(f'bridge delivery failed; sequence={next_sequence}, count={len(events)}') from last_error

def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--config', required=True)
    parser.add_argument('--batch', type=int, default=1, help='default 1 sends low-rate live input immediately; larger values wait for that many records or EOF')
    args = parser.parse_args()
    if not 1 <= args.batch <= 512:
        parser.error('--batch must be 1..512')
    agent = Agent(args.config)
    pending = []
    for line in sys.stdin:
        if not line.strip():
            continue
        row = json.loads(line)
        if not isinstance(row, dict) or 'kind' not in row:
            raise ValueError('event object with kind required')
        pending.append(row)
        if len(pending) >= args.batch:
            print(json.dumps(agent.submit(pending)), flush=True)
            pending = []
    if pending:
        print(json.dumps(agent.submit(pending)), flush=True)

if __name__ == '__main__':
    main()
