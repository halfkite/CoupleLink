import type { CapacitorConfig } from '@capacitor/cli';
const config: CapacitorConfig = {
  appId: 'app.couplelink.mobile', appName: 'CoupleLink', webDir: 'dist',
  loggingBehavior: 'none',
  android: { allowMixedContent: true, webContentsDebuggingEnabled: false },
};
export default config;
