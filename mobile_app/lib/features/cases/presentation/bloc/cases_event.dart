sealed class CasesEvent {
  const CasesEvent();
}

class CasesRequested extends CasesEvent {
  const CasesRequested();
}

class CasesNextPage extends CasesEvent {
  const CasesNextPage();
}

class CasesFilterChanged extends CasesEvent {
  const CasesFilterChanged({required this.mine, this.status});
  final bool mine;
  final String? status;
}
