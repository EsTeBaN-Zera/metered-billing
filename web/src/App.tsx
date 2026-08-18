import { useEffect, useState } from "react";
import CustomerPage from "./Customer";
import OpsPage from "./Ops";

export default function App() {
  const [page, setPage] = useState(window.location.hash === "#/ops" ? "ops" : "customer");
  useEffect(() => {
    const onHash = () => setPage(window.location.hash === "#/ops" ? "ops" : "customer");
    window.addEventListener("hashchange", onHash);
    return () => window.removeEventListener("hashchange", onHash);
  }, []);
  return (
    <>
      <header>
        <strong>Billing</strong>
        <a href="#/">Customer</a>
        <a href="#/ops">Ops</a>
      </header>
      <main>{page === "ops" ? <OpsPage /> : <CustomerPage />}</main>
    </>
  );
}
