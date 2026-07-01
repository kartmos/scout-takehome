import { Provider } from 'react-redux';
import { store } from './store';
import { configResult } from '../shared/config/env';
import { GalleryPage } from '../pages/gallery/GalleryPage';
import styles from './App.module.css';
import '../shared/styles/globals.css';

function ConfigError({ message }: { message: string }) {
  return (
    <div className={styles.configError} role="alert">
      <h1>Scout — Configuration Error</h1>
      <p>{message}</p>
      <p>
        Copy <code>.env.example</code> to <code>.env</code> in the project root and set the required
        values, then restart the dev server.
      </p>
    </div>
  );
}

function Shell() {
  return (
    <div className={styles.shell}>
      <header className={styles.header}>
        <h1>Scout</h1>
      </header>
      <main className={styles.main}>
        <GalleryPage />
      </main>
    </div>
  );
}

export function App() {
  if (!configResult.ok) {
    return <ConfigError message={configResult.error} />;
  }

  return (
    <Provider store={store}>
      <Shell />
    </Provider>
  );
}
