package app.couplelink.mobile;

/** One policy for foreground and background interaction notifications. */
final class InteractionAlert {
    /** v2 adds an explicit vibration pattern; channel settings are immutable once created. */
    static final String CHANNEL = "couplelink_alerts_v2";
    /** Shared by the notification channel and the in-app feedback button. */
    static final long[] VIBRATION_PATTERN = {0, 250, 150, 250};
    final boolean sound, vibrate;
    final String channel;
    private InteractionAlert(boolean sound, boolean vibrate, String channel) {
        this.sound = sound; this.vibrate = vibrate; this.channel = channel;
    }
    static InteractionAlert of(String action, boolean allowSound, boolean allowVibrate, boolean fresh) {
        if (!java.util.Arrays.asList("poke", "miss_you", "hug", "vibrate", "ring").contains(action)) throw new IllegalArgumentException("Unknown interaction");
        boolean sound = fresh && action.equals("ring") && allowSound;
        boolean vibrate = fresh && !action.equals("ring") && allowVibrate;
        return new InteractionAlert(sound, vibrate, CHANNEL);
    }
}
