export function centsToCurrency(value: number | null | undefined): string {
  const cents = value ?? 0;
  return new Intl.NumberFormat('pt-BR', {
    style: 'currency',
    currency: 'BRL',
  }).format(cents / 100);
}

export function centsToCurrencyAbsolute(value: number | null | undefined): string {
  return centsToCurrency(Math.abs(value ?? 0));
}

export function centsToCompactCurrencyAbsolute(value: number | null | undefined): string {
  return new Intl.NumberFormat('pt-BR', {
    minimumFractionDigits: 2,
    maximumFractionDigits: 2,
  }).format(Math.abs(value ?? 0) / 100);
}

export function centsToDecimal(value: number | null | undefined): string {
  return ((value ?? 0) / 100).toFixed(2);
}

export function decimalToCents(value: string | number | null | undefined): number {
  if (value === null || value === undefined || value === '') {
    return 0;
  }

  const normalized = String(value)
    .trim()
    .replace(/\./g, '')
    .replace(',', '.');

  const parsed = Number(normalized);
  if (Number.isNaN(parsed)) {
    return 0;
  }

  return Math.round(parsed * 100);
}

export function toDateInputValue(value: string | Date | null | undefined): string {
  if (!value) {
    return '';
  }
  return new Date(value).toISOString().slice(0, 10);
}

export function dateInputToIso(value: string): string {
  return new Date(`${value}T00:00:00Z`).toISOString();
}

export function toBrazilianDateInputValue(value: string | Date | null | undefined): string {
  const isoDate = toDateInputValue(value);
  if (!isoDate) {
    return '';
  }

  const [year, month, day] = isoDate.split('-');
  return `${day}/${month}/${year}`;
}

export function brazilianDateToQuery(value: string | null | undefined): string {
  if (!value) {
    return '';
  }

  const normalized = value.trim();
  const match = /^(\d{2})\/(\d{2})\/(\d{4})$/.exec(normalized);
  if (!match) {
    return normalized;
  }

  const [, day, month, year] = match;
  return `${year}-${month}-${day}`;
}

export function toBrazilianDate(value: string | Date | null | undefined): string {
  if (!value) {
    return '';
  }
  return new Intl.DateTimeFormat('pt-BR', {
    day: '2-digit',
    month: 'short',
    year: 'numeric',
    timeZone: 'UTC',
  }).format(new Date(value)).replaceAll(' de ', ' / ');
}
