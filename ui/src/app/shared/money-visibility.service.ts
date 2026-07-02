import { Injectable, signal } from '@angular/core';

import { centsToCompactCurrencyAbsolute, centsToCurrency, centsToCurrencyAbsolute } from './money';

const MASK = '••••';

@Injectable({ providedIn: 'root' })
export class MoneyVisibilityService {
  readonly hidden = signal(false);

  toggle(): void {
    this.hidden.update((value) => !value);
  }

  setHidden(hidden: boolean): void {
    this.hidden.set(hidden);
  }

  formatCurrency(value: number | null | undefined): string {
    return this.hidden() ? `R$ ${MASK}` : centsToCurrency(value);
  }

  formatCurrencyAbsolute(value: number | null | undefined): string {
    return this.hidden() ? `R$ ${MASK}` : centsToCurrencyAbsolute(value);
  }

  formatCompactCurrencyAbsolute(value: number | null | undefined): string {
    return this.hidden() ? MASK : centsToCompactCurrencyAbsolute(value);
  }

  formatSignedCurrency(value: number | null | undefined): string {
    const amount = value ?? 0;
    if (this.hidden()) {
      return amount < 0 ? `- ${MASK}` : MASK;
    }

    const absolute = centsToCurrencyAbsolute(amount);
    return amount < 0 ? `- ${absolute}` : absolute;
  }

}
