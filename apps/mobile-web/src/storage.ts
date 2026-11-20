import type { Cache, ChatCache } from './core';

type Session = { origin: string; user: unknown; couple: unknown; savedAt: number };

export class ChatStorage implements Cache {
  private db: Promise<IDBDatabase>;
  constructor() {
    this.db = new Promise((resolve, reject) => {
      const request = indexedDB.open('couplelink-web-v1', 3);
      request.onupgradeneeded = () => {
        const db = request.result;
        if (!db.objectStoreNames.contains('chats')) db.createObjectStore('chats', { keyPath: 'scope' });
        if (!db.objectStoreNames.contains('images')) db.createObjectStore('images');
        if (!db.objectStoreNames.contains('session')) db.createObjectStore('session', { keyPath: 'origin' });
        if (!db.objectStoreNames.contains('notes')) db.createObjectStore('notes', { keyPath: 'scope' });
      };
      request.onsuccess = () => resolve(request.result);
      request.onerror = () => reject(new Error('storage_failed'));
    });
  }
  private run<T>(store: string, mode: IDBTransactionMode, action: (store: IDBObjectStore) => IDBRequest): Promise<T> {
    return this.db.then(db => new Promise<T>((resolve, reject) => {
      const tx = db.transaction(store, mode), request = action(tx.objectStore(store));
      tx.oncomplete = () => resolve(request.result as T);
      tx.onabort = tx.onerror = () => reject(new Error('storage_failed'));
    }));
  }
  async load(scope: string): Promise<ChatCache> {
    return (await this.run<ChatCache>('chats', 'readonly', s => s.get(scope))) || { scope, cursor: 0, read: 0, messages: [], draft: '' };
  }
  save(value: ChatCache): Promise<void> {
    // Serialize Vue proxies before awaiting; each transaction stores a coherent snapshot.
    const snapshot = JSON.parse(JSON.stringify(value));
    return this.run<unknown>('chats', 'readwrite', s => s.put(snapshot)).then(() => undefined);
  }
  async loadSession(origin: string): Promise<{ user: unknown; couple: unknown } | null> {
    try {
      const saved = await this.run<Session>('session', 'readonly', s => s.get(origin));
      return saved ? { user: saved.user, couple: saved.couple } : null;
    } catch { return null; }
  }
  async saveSession(origin: string, user: unknown, couple: unknown): Promise<void> {
    try { await this.run<unknown>('session', 'readwrite', s => s.put({ origin, user, couple, savedAt: Date.now() })); } catch { /* cache is best effort */ }
  }
  async loadNotes(scope: string): Promise<unknown[] | null> {
    try {
      const saved = await this.run<{ scope: string; notes: unknown[] }>('notes', 'readonly', s => s.get(scope));
      return saved && Array.isArray(saved.notes) ? saved.notes : null;
    } catch { return null; }
  }
  async saveNotes(scope: string, notes: unknown[]): Promise<void> {
    try { await this.run<unknown>('notes', 'readwrite', s => s.put({ scope, notes: JSON.parse(JSON.stringify(notes)), savedAt: Date.now() })); } catch { /* cache is best effort */ }
  }
  async loadImage(id: string): Promise<Blob | null> {
    try { return (await this.run<Blob>('images', 'readonly', s => s.get(id))) || null; } catch { return null; }
  }
  async saveImage(id: string, blob: Blob): Promise<void> {
    try { await this.run<unknown>('images', 'readwrite', s => s.put(blob, id)); } catch { /* cache is best effort */ }
  }
}