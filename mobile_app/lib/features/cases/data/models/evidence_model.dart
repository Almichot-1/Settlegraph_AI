import '../../../../core/models/money.dart';

class EvidenceModel {
  const EvidenceModel({
    required this.id,
    required this.recordType,
    required this.reference,
    required this.occurredAt,
    this.amount,
    this.providerId,
    this.payloadHash,
  });
  factory EvidenceModel.fromJson(Map<String, dynamic> j) => EvidenceModel(
    id: j['id'] as String,
    recordType: j['record_type'] as String,
    reference: j['reference'] as String,
    occurredAt: DateTime.parse(j['occurred_at'] as String),
    providerId: j['provider_id'] as String?,
    amount: j['amount'] == null
        ? null
        : Money.fromJson(j['amount'] as Map<String, dynamic>),
    payloadHash: j['raw_payload_sha256'] as String?,
  );
  final String id, recordType, reference;
  final DateTime occurredAt;
  final Money? amount;
  final String? providerId, payloadHash;
  String get label => switch (recordType) {
    'ORDER' => 'Order',
    'PAYMENT' => 'Payment',
    'SETTLEMENT' => 'Settlement',
    'SOURCE_STATUS' => 'Source coverage',
    _ => 'Unknown record',
  };
}
