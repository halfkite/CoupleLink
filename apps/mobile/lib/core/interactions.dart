import 'package:flutter/material.dart';

const remoteActionIds = ['miss_you', 'poke', 'hug', 'ring'];
const actionIcons = <String, IconData>{
  'miss_you': Icons.favorite_rounded,
  'poke': Icons.touch_app_rounded,
  'hug': Icons.diversity_1_rounded,
  'ring': Icons.notifications_active_rounded,
  'vibrate': Icons.vibration_rounded,
};
const actionKeys = <String, String>{
  'miss_you': 'interaction.miss',
  'poke': 'interaction.poke',
  'hug': 'interaction.hug',
  'ring': 'interaction.ring',
  'vibrate': 'feedback.vibrate',
};
