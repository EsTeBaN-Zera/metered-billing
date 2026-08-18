import { useMemo, useState } from "react";
import { api, money, monthRange } from "./api";

type UsageItem = { api_key_id: string; hour: string; units: number };
type Invoice = {
  id: string;
  status: string;
  total_micros: number;
  period_start: string;
  line_items?: { description: string; amount_micros: number }[];
};

export default function CustomerPage() {
  const [key, setKey] = useState(localStorage.getItem("apiKey") || "");
  const [err, setErr] = useState("");
  const [loading, setLoading] = useState(false);
  const [usage, setUsage] = useState<UsageItem[]>([]);
  const [invoices, setInvoices] = useState<Invoice[]>([]);
  const [detail, setDetail] = useState<Invoice | null>(null);

  async function load() {
    setErr("");
    setLoading(true);
    localStorage.setItem("apiKey", key);
    const { from, to } = monthRange();
    try {
      const u = await api(`/v1/usage?from=${from}&to=${to}&limit=200`, key);
      const i = await api("/v1/invoices", key);
      if (!u.ok || !i.ok) {
        setErr("Could not load. Check the API key.");
        return;
      }
      const uj = await u.json();
      const ij = await i.json();
      setUsage(uj.items || []);
      setInvoices(ij.items || []);
    } catch {
      setErr("Network error");
    } finally {
      setLoading(false);
    }
  }

  const days = useMemo(() => {
    const map = new Map<string, number>();
    for (const row of usage) {
      const day = row.hour.slice(0, 10);
      map.set(day, (map.get(day) || 0) + row.units);
    }
    return [...map.entries()].sort();
  }, [usage]);
  const max = Math.max(1, ...days.map((d) => d[1]));

  async function openInvoice(id: string) {
    setErr("");
    const r = await api("/v1/invoices/" + id, key);
    if (!r.ok) {
      setErr("Invoice not found");
      return;
    }
    setDetail(await r.json());
  }

  return (
    <>
      <div className="card">
        <h2>Customer</h2>
        <div className="row">
          <input
            style={{ flex: 1 }}
            placeholder="API key sk_live_..."
            value={key}
            onChange={(e) => setKey(e.target.value)}
          />
          <button onClick={load} disabled={loading}>
            {loading ? "Loading..." : "Load"}
          </button>
        </div>
        {err && <p className="error">{err}</p>}
      </div>
      <div className="card">
        <h3>Current month usage</h3>
        {days.length === 0 ? (
          <p>No windows yet.</p>
        ) : (
          <div className="bars">
            {days.map(([day, units]) => (
              <div
                key={day}
                className="bar"
                title={`${day}: ${units}`}
                style={{ height: `${(units / max) * 100}%` }}
              />
            ))}
          </div>
        )}
      </div>
      <div className="card">
        <h3>Invoices</h3>
        <table>
          <thead>
            <tr>
              <th>Period</th>
              <th>Status</th>
              <th>Total</th>
            </tr>
          </thead>
          <tbody>
            {invoices.map((inv) => (
              <tr key={inv.id} onClick={() => openInvoice(inv.id)} style={{ cursor: "pointer" }}>
                <td>{String(inv.period_start).slice(0, 10)}</td>
                <td>{inv.status}</td>
                <td>{money(inv.total_micros)}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
      {detail && (
        <div className="card">
          <h3>Invoice {detail.id.slice(0, 8)}...</h3>
          <p>
            {detail.status} / {money(detail.total_micros)}
          </p>
          <table>
            <tbody>
              {(detail.line_items || []).map((l, i) => (
                <tr key={i}>
                  <td>{l.description}</td>
                  <td>{money(l.amount_micros)}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </>
  );
}
