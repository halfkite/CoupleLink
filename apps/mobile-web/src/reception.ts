export interface ReceptionDependencies {
  context(): { scope: string; foreground: boolean };
  status(): Promise<{ running: boolean; notificationsEnabled: boolean }>;
  permission(): Promise<{ granted: boolean }>;
  start(): Promise<void>;
  stop(): Promise<void>;
  wasPrompted(): boolean;
  markPrompted(): void;
  update(running: boolean): void;
}
/** Permission and background reception form one user action, never two switches. */
export class AutoReception {
  private busy = false;
  constructor(private deps: ReceptionDependencies) {}
  async ensure(explicit = false): Promise<boolean | undefined> {
    const context = this.deps.context();
    if (this.busy || !context.foreground || (!context.scope && !explicit)) return;
    const current = () => this.deps.context().scope === context.scope && this.deps.context().foreground;
    this.busy = true;
    try {
      const status = await this.deps.status();
      if (!current()) return;
      this.deps.update(status.running);
      let allowed = status.notificationsEnabled;
      // Remember an existing grant too: revoking it later in system settings
      // must not be mistaken for the first permission request.
      if (allowed) this.deps.markPrompted();
      if (!allowed && (explicit || !this.deps.wasPrompted())) {
        this.deps.markPrompted();
        allowed = (await this.deps.permission()).granted;
        if (!current()) return;
      }
      if (!allowed) {
        if (status.running) await this.deps.stop();
        if (current()) this.deps.update(false);
        return false;
      }
      if (context.scope && !status.running) {
        await this.deps.start();
        if (!current()) { await this.deps.stop(); return; }
      }
      this.deps.update(context.scope ? true : status.running);
      return true;
    } finally { this.busy = false; }
  }
}
