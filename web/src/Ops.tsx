import { useState } from "react";
import { api, money } from "./api";

type Customer = { id: string; name: string };
type Detail = {
  id: string;
  name: string;
  anomaly: boolean;
  today_units: number;
  avg_30_units: number;
  invoices: { id: string; status: string; total_micros: number; period_start: string }[];
};
type Line = { id: string; description: string; amount_micros: number };
type Inv = { id: string; status: string; total_micros: number; line_items?: Line[] };

export default function OpsPage() {
  const [token, setToken] = useState(localStorage.getItem("opsToken") || "");
  const [err, setErr] = useState("");
  const [loading, setLoading] = useState(false);
  const [customers, setCustomers] = useState<Customer[]>([]);
  const [detail, setDetail] = useState<Detail | null>(null);
  const [invoice, setInvoice] = useState<Inv | null>(null);
  const [creditAmt, setCreditAmt] = useState("20");
  const [creditReason, setCreditReason] = useState("");
  const [confirmCredit, setConfirmCredit] = useState(false);
  const [lineId, setLineId] = useState("");
  const [lineAmt, setLineAmt] = useState("");
  const [lineReason, setLineReason] = useState("");
  const [confirmLine, setConfirmLine] = useState(false);
  const [busy, setBusy] = useState(false);
  const [ok, setOk] = useState("");
  const [opening, setOpening] = useState("");
  const [showTests, setShowTests] = useState(false);

  const visible = customers.filter((c) => showTests || (c.name || "").startsWith("seed:"));

  async function list() {
    setErr("");
    setOk("");
    setLoading(true);
    localStorage.setItem("opsToken", token);
    try {
      const r = await api("/ops/customers?limit=200", token);
      if (!r.ok) {
        setErr("Ops token rejected");
        return;
      }
      const j = await r.json();
      setCustomers(j.items || []);
    } catch {
      setErr("Network error");
    } finally {
      setLoading(false);
    }
  }

  async function openCustomer(id: string) {
    setInvoice(null);
    setErr("");
    setOpening(id);
    try {
      const r = await api("/ops/customers/" + id, token);
      if (!r.ok) {
        setErr("Customer not found (" + r.status + ")");
        return;
      }
      const d = await r.json();
      setDetail(d);
    } catch (e) {
      setErr("Failed to open customer");
    } finally {
      setOpening("");
    }
  }

  async function openInvoice(id: string) {
    const r = await api("/ops/invoices/" + id, token);
    if (!r.ok) {
      setErr("Invoice not found");
      return;
    }
    const inv = await r.json();
    setInvoice(inv);
    const first = (inv.line_items || [])[0];
    if (first) {
      setLineId(first.id);
      setLineAmt(String(first.amount_micros / 1_000_000));
    }
  }

  async function doCredit() {
    if (!detail) return;
    setBusy(true);
    setErr("");
    setOk("");
    const r = await api("/ops/customers/" + detail.id + "/credits", token, {
      method: "POST",
      headers: { "Idempotency-Key": crypto.randomUUID() },
      body: JSON.stringify({
        amount_micros: Math.round(Number(creditAmt) * 1_000_000),
        reason: creditReason,
      }),
    });
    setBusy(false);
    setConfirmCredit(false);
    if (!r.ok) {
      setErr(await r.text());
      return;
    }
    setOk("Credit saved");
    setCreditReason("");
    openCustomer(detail.id);
  }

  async function doOverride() {
    if (!invoice) return;
    setBusy(true);
    setErr("");
    setOk("");
    const r = await api(
      `/ops/invoices/${invoice.id}/line-items/${lineId}`,
      token,
      {
        method: "PATCH",
        body: JSON.stringify({
          amount_micros: Math.round(Number(lineAmt) * 1_000_000),
          reason: lineReason,
        }),
      }
    );
    setBusy(false);
    setConfirmLine(false);
    if (!r.ok) {
      setErr(await r.text());
      return;
    }
    setOk("Line updated");
    setLineReason("");
    openInvoice(invoice.id);
  }

  return (
    <>
      <div className="card">
        <h2>Ops</h2>
        <div className="row">
          <input
            style={{ flex: 1 }}
            placeholder="OPS_TOKEN"
            value={token}
            onChange={(e) => setToken(e.target.value)}
          />
          <button onClick={list} disabled={loading}>
            {loading ? "Loading..." : "Load customers"}
          </button>
        </div>
        <label className="row">
          <input type="checkbox" checked={showTests} onChange={(e) => setShowTests(e.target.checked)} />
          Show test customers
        </label>
        {err && <p className="error">{err}</p>}
        {ok && <p className="ok">{ok}</p>}
        {opening && <p>Opening...</p>}
      </div>
      {detail && (
        <div className="card">
          <h3>
            {detail.name} {detail.anomaly && <span className="badge">10x usage</span>}
          </h3>
          <p>
            Today {detail.today_units ?? 0} units / 30-day avg{" "}
            {Number(detail.avg_30_units || 0).toFixed(0)}
          </p>
          <h4>Credit</h4>
          <div className="row">
            <input
              type="number"
              value={creditAmt}
              onChange={(e) => setCreditAmt(e.target.value)}
              placeholder="USD"
            />
            <input
              placeholder="reason"
              value={creditReason}
              onChange={(e) => setCreditReason(e.target.value)}
            />
            <button onClick={() => setConfirmCredit(true)} disabled={!creditReason}>
              Issue credit
            </button>
          </div>
          <h4>Invoices</h4>
          <table>
            <tbody>
              {(detail.invoices || []).map((inv) => (
                <tr key={inv.id} onClick={() => openInvoice(inv.id)} style={{ cursor: "pointer" }}>
                  <td>{String(inv.period_start).slice(0, 10)}</td>
                  <td>{inv.status}</td>
                  <td>{money(inv.total_micros)}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
      {invoice && (
        <div className="card">
          <h3>
            Invoice {invoice.status} / {money(invoice.total_micros)}
          </h3>
          <table>
            <tbody>
              {(invoice.line_items || []).map((l) => (
                <tr key={l.id}>
                  <td>{l.description}</td>
                  <td>{money(l.amount_micros)}</td>
                  <td>
                    <button onClick={() => { setLineId(l.id); setLineAmt(String(l.amount_micros / 1_000_000)); }}>
                      Select
                    </button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
          <h4>Override line</h4>
          <div className="row">
            <input value={lineAmt} onChange={(e) => setLineAmt(e.target.value)} />
            <input
              placeholder="reason"
              value={lineReason}
              onChange={(e) => setLineReason(e.target.value)}
            />
            <button onClick={() => setConfirmLine(true)} disabled={!lineReason || invoice.status === "paid"}>
              Override
            </button>
          </div>
        </div>
      )}
      <div className="card">
        <h3>Customers</h3>
        <table>
          <tbody>
            {visible.length === 0 && (
              <tr>
                <td colSpan={2}>No seed customers. Try Show test customers.</td>
              </tr>
            )}
            {visible.map((c) => (
              <tr
                key={c.id}
                className="clickable"
                onClick={() => openCustomer(c.id)}
                style={{
                  background: detail?.id === c.id ? "#dce6f0" : undefined,
                }}
              >
                <td>{c.name}</td>
                <td>{(c.id || "").slice(0, 8)}...</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
      {confirmCredit && (
        <div className="modal">
          <div className="card">
            <p>
              Credit ${creditAmt} to {detail?.name}?
            </p>
            <div className="row">
              <button onClick={doCredit} disabled={busy}>
                {busy ? "Saving..." : "Confirm"}
              </button>
              <button onClick={() => setConfirmCredit(false)}>Cancel</button>
            </div>
          </div>
        </div>
      )}
      {confirmLine && (
        <div className="modal">
          <div className="card">
            <p>Change line to ${lineAmt}?</p>
            <div className="row">
              <button onClick={doOverride} disabled={busy}>
                {busy ? "Saving..." : "Confirm"}
              </button>
              <button onClick={() => setConfirmLine(false)}>Cancel</button>
            </div>
          </div>
        </div>
      )}
    </>
  );
}
