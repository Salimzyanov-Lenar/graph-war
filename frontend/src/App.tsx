import { useEffect, useState } from 'react';
import { createDemoSession, type DemoSession } from './demo/demoSession';
import { MatchScreen } from './ui/MatchScreen';
import { OnlineApp } from './ui/OnlineApp';

const isDemo = new URLSearchParams(window.location.search).has('demo');

function Demo() {
  const [session, setSession] = useState<DemoSession>();

  useEffect(() => {
    const next = createDemoSession();
    setSession(next);
    return () => next.dispose();
  }, []);

  return session ? <MatchScreen store={session.store} send={session.send} /> : null;
}

export function App() {
  return (
    <main className="app">
      <h1>
        Graphwar {isDemo && <small>демо на моках, без сервера</small>}
      </h1>
      {isDemo ? <Demo /> : <OnlineApp />}
    </main>
  );
}
