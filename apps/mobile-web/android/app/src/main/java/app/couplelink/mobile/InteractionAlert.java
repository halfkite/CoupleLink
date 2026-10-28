package app.couplelink.mobile;

/** One policy for foreground and background interaction notifications. */
final class InteractionAlert {
    static final String CHANNEL = "couplelink_alerts_v1";
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
