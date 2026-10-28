import type { Cache, ChatCache } from './core';
export class ChatStorage implements Cache {
  private db: Promise<IDBDatabase>;
  constructor() {
    this.db = new Promise((resolve, reject) => {
      const request = indexedDB.open('couplelink-web-v1', 1);
      request.onupgradeneeded = () => request.result.createObjectStore('chats', { keyPath: 'scope' });
      request.onsuccess = () => resolve(request.result);
      request.onerror = () => reject(new Error('storage_failed'));
    });
  }
  async load(scope: string): Promise<ChatCache> {
    const db = await this.db;
    return new Promise((resolve, reject) => {
      const tx = db.transaction('chats', 'readonly'), req = tx.objectStore('chats').get(scope);
      tx.oncomplete = () => resolve(req.result || { scope, cursor: 0, read: 0, messages: [], draft: '' });
      tx.onabort = tx.onerror = () => reject(new Error('storage_failed'));
    });
  }
  save(value: ChatCache): Promise<void> {
    // Serialize Vue proxies before awaiting; each transaction stores a coherent snapshot.
    const snapshot = JSON.parse(JSON.stringify(value));
    return this.db.then(db => new Promise((resolve, reject) => {
      const tx = db.transaction('chats', 'readwrite'); tx.objectStore('chats').put(snapshot);
      tx.oncomplete = () => resolve(); tx.onabort = tx.onerror = () => reject(new Error('storage_failed'));
    }));
  }
}
