import { useEffect, useState } from 'react';
import { createDemoSession, type DemoSession } from './demo/demoSession';
import { MatchScreen } from './ui/MatchScreen';

export function App() {
  const [session, setSession] = useState<DemoSession>();

  useEffect(() => {
    const next = createDemoSession();
    setSession(next);
    return () => next.dispose();
  }, []);

  return (
    <main className="app">
      <h1>Graphwar <small>демо на моках, без сервера</small></h1>
      {session && <MatchScreen store={session.store} send={session.send} />}
    </main>
  );
}
