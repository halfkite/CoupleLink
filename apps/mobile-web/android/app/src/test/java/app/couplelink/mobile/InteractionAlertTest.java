package app.couplelink.mobile;

import static org.junit.Assert.*;
import org.junit.Test;

public class InteractionAlertTest {
    @Test public void touchInteractionsUseVibrationInsteadOfTheOldQuietCategory() {
        for (String action : new String[]{"poke", "miss_you", "hug", "vibrate"}) {
            InteractionAlert alert = InteractionAlert.of(action, true, true, true);
            assertTrue(alert.vibrate); assertFalse(alert.sound);
            assertEquals(InteractionAlert.CHANNEL, alert.channel);
        }
    }
    @Test public void RingRequiresItsOwnReceivingPermission() {
        InteractionAlert denied = InteractionAlert.of("ring", false, true, true);
        assertFalse(denied.sound); assertFalse(denied.vibrate);
        InteractionAlert allowed = InteractionAlert.of("ring", true, false, true);
        assertTrue(allowed.sound); assertFalse(allowed.vibrate);
        assertEquals(InteractionAlert.CHANNEL, allowed.channel);
        assertEquals(InteractionAlert.CHANNEL, denied.channel);
    }
    @Test public void ExpiredEventsCannotTriggerEitherFeedback() {
        for (String action : new String[]{"poke", "miss_you", "hug", "vibrate", "ring"}) {
            InteractionAlert alert = InteractionAlert.of(action, true, true, false);
            assertFalse(alert.sound); assertFalse(alert.vibrate);
            assertEquals(InteractionAlert.CHANNEL, alert.channel);
        }
    }
    @Test public void DisabledReceivingPermissionRemainsQuiet() {
        InteractionAlert alert = InteractionAlert.of("vibrate", true, false, true);
        assertFalse(alert.sound); assertFalse(alert.vibrate);
    }
    @Test(expected = IllegalArgumentException.class) public void UnknownActionCannotBecomeAnAlert() {
        InteractionAlert.of("invalid", true, true, true);
    }
}
