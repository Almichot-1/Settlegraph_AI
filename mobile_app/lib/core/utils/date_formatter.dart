import 'package:intl/intl.dart';

String formatDate(DateTime date) =>
    DateFormat('d MMM, HH:mm').format(date.toLocal());
String formatFullDate(DateTime date) =>
    '${DateFormat('d MMM yyyy, HH:mm').format(date.toLocal())} · local time';
