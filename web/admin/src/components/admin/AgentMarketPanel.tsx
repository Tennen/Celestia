import { useEffect, useState } from 'react';
import { Play, Plus, Save, Trash2 } from 'lucide-react';
import { Button } from '../ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '../ui/card';
import { Textarea } from '../ui/textarea';
import {
  importMarketPortfolioCodes,
  runMarketAnalysis,
  saveMarketPortfolio,
  type AgentMarketPortfolio,
  type AgentSnapshot,
} from '../../lib/agent';
import { Field, FieldGrid, SelectField, numberValue, parseOptionalNumber, requiredNumber } from './AgentFormFields';
import type { AgentRunner } from './AgentWorkspace';

type Props = {
  snapshot: AgentSnapshot;
  onRun: AgentRunner;
};

type Holding = AgentMarketPortfolio['funds'][number];

const marketPhases = [
  { value: 'midday', label: 'midday' },
  { value: 'close', label: 'close' },
];

export function AgentMarketPanel({ snapshot, onRun }: Props) {
  const [cash, setCash] = useState(numberValue(snapshot.market.portfolio.cash));
  const [holding, setHolding] = useState<Holding>(snapshot.market.portfolio.funds[0] ?? { code: '', name: '' });
  const [marketCodes, setMarketCodes] = useState('');
  const [marketPhase, setMarketPhase] = useState('close');
  const [marketNotes, setMarketNotes] = useState('');

  useEffect(() => {
    setCash(numberValue(snapshot.market.portfolio.cash));
    setHolding(snapshot.market.portfolio.funds[0] ?? { code: '', name: '' });
  }, [snapshot]);

  const saveHolding = () => {
    const nextHolding: Holding = {
      code: holding.code.trim(),
      name: holding.name.trim(),
      quantity: parseOptionalNumber(String(holding.quantity ?? '')),
      avg_cost: parseOptionalNumber(String(holding.avg_cost ?? '')),
    };
    const funds = snapshot.market.portfolio.funds.some((item) => item.code === nextHolding.code)
      ? snapshot.market.portfolio.funds.map((item) => (item.code === nextHolding.code ? nextHolding : item))
      : [...snapshot.market.portfolio.funds, nextHolding];
    onRun('market-save', () => saveMarketPortfolio({ ...snapshot.market.portfolio, cash: requiredNumber(cash), funds }), false);
  };

  return (
    <div className="grid grid--two">
      <Card className="panel">
        <CardHeader>
          <CardTitle>Market Portfolio</CardTitle>
          <CardDescription>{snapshot.market.portfolio.funds.length} holdings, {snapshot.market.runs.length} runs</CardDescription>
        </CardHeader>
        <CardContent className="stack">
          <Field label="Cash" value={cash} onChange={setCash} />
          <div className="button-row">
            {snapshot.market.portfolio.funds.map((item) => (
              <Button key={item.code} variant={item.code === holding.code ? 'default' : 'secondary'} onClick={() => setHolding(item)}>
                {item.name || item.code}
              </Button>
            ))}
            <Button variant="secondary" onClick={() => setHolding({ code: '', name: '' })}>
              <Plus className="mr-2 h-4 w-4" />
              New
            </Button>
          </div>
          <FieldGrid>
            <Field label="Code" value={holding.code} onChange={(code) => setHolding({ ...holding, code })} />
            <Field label="Name" value={holding.name} onChange={(name) => setHolding({ ...holding, name })} />
            <Field label="Quantity" value={String(holding.quantity ?? '')} onChange={(quantity) => setHolding({ ...holding, quantity: parseOptionalNumber(quantity) })} />
            <Field label="Average cost" value={String(holding.avg_cost ?? '')} onChange={(avg_cost) => setHolding({ ...holding, avg_cost: parseOptionalNumber(avg_cost) })} />
          </FieldGrid>
          <div className="button-row">
            <Button onClick={saveHolding} disabled={!holding.code.trim()}>
              <Save className="mr-2 h-4 w-4" />
              Save Holding
            </Button>
            <Button
              variant="danger"
              onClick={() => onRun('market-save', () => saveMarketPortfolio({ ...snapshot.market.portfolio, funds: snapshot.market.portfolio.funds.filter((item) => item.code !== holding.code) }), false)}
              disabled={!holding.code}
            >
              <Trash2 className="mr-2 h-4 w-4" />
              Delete
            </Button>
          </div>
        </CardContent>
      </Card>

      <Card className="panel">
        <CardHeader>
          <CardTitle>Market Run</CardTitle>
          <CardDescription>Fund analysis uses LLM, Search Engine, markdown report, and WeCom image pipeline internally</CardDescription>
        </CardHeader>
        <CardContent className="stack">
          <Textarea value={marketCodes} onChange={(event) => setMarketCodes(event.target.value)} placeholder="Fund or security codes, separated by commas or lines" />
          <Button variant="secondary" onClick={() => onRun('market-import', () => importMarketPortfolioCodes({ codes: marketCodes }))} disabled={!marketCodes.trim()}>
            Import Codes
          </Button>
          <SelectField label="Phase" value={marketPhase} options={marketPhases} onChange={setMarketPhase} />
          <Textarea value={marketNotes} onChange={(event) => setMarketNotes(event.target.value)} placeholder="Run notes" />
          <Button onClick={() => onRun('market-run', () => runMarketAnalysis({ phase: marketPhase, notes: marketNotes }))}>
            <Play className="mr-2 h-4 w-4" />
            Run Analysis
          </Button>
        </CardContent>
      </Card>
    </div>
  );
}
