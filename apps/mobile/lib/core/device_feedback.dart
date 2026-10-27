import 'package:flutter/foundation.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

enum FeedbackOutcome { requested, suppressed, unsupported, failed }

abstract interface class DeviceFeedback {
  Future<FeedbackOutcome> preview({required bool sound});
  Future<void> stop();
}

final deviceFeedbackProvider = Provider<DeviceFeedback>(
  (ref) => AndroidDeviceFeedback(),
);

class AndroidDeviceFeedback implements DeviceFeedback {
  static const channel = MethodChannel('app.couplelink/feedback');
  @override
  Future<FeedbackOutcome> preview({required bool sound}) async {
    if (kIsWeb || defaultTargetPlatform != TargetPlatform.android) {
      return FeedbackOutcome.unsupported;
    }
    try {
      final result = await channel.invokeMethod<String>(
        sound ? 'previewSound' : 'previewVibration',
      );
      return switch (result) {
        'requested' => FeedbackOutcome.requested,
        'suppressed' => FeedbackOutcome.suppressed,
        'unsupported' => FeedbackOutcome.unsupported,
        _ => FeedbackOutcome.failed,
      };
    } on MissingPluginException {
      return FeedbackOutcome.unsupported;
    } on PlatformException {
      return FeedbackOutcome.failed;
    }
  }

  @override
  Future<void> stop() async {
    if (kIsWeb || defaultTargetPlatform != TargetPlatform.android) return;
    try {
      await channel.invokeMethod<void>('stop');
    } on MissingPluginException {
      return;
    } on PlatformException catch (error, stack) {
      FlutterError.reportError(
        FlutterErrorDetails(
          exception: error,
          stack: stack,
          library: 'CoupleLink local feedback stop',
        ),
      );
    }
  }
}
