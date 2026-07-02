import { brazilianDateToQuery, centsToCompactCurrencyAbsolute, centsToCurrencyAbsolute, centsToDecimal, dateInputToIso, decimalToCents, toBrazilianDate, toBrazilianDateInputValue } from './money';

describe('money helpers', () => {
  it('converts decimal strings to cents', () => {
    expect(decimalToCents('123,45')).toBe(12345);
    expect(decimalToCents('1.234,56')).toBe(123456);
    expect(decimalToCents('12.34')).toBe(123400);
  });

  it('converts cents to decimal strings', () => {
    expect(centsToDecimal(12345)).toBe('123.45');
  });

  it('converts date inputs to backend ISO dates', () => {
    expect(dateInputToIso('2026-01-02')).toBe('2026-01-02T00:00:00.000Z');
  });

  it('formats iso dates for brazilian text inputs', () => {
    expect(toBrazilianDateInputValue('2026-01-02T00:00:00Z')).toBe('02/01/2026');
  });

  it('formats dates with abbreviated month names', () => {
    expect(toBrazilianDate('2026-01-02T00:00:00Z')).toBe('02 / jan. / 2026');
  });

  it('converts Brazilian filter dates to backend query dates', () => {
    expect(brazilianDateToQuery('02/01/2026')).toBe('2026-01-02');
  });

  it('formats currency without the sign for row displays', () => {
    expect(centsToCurrencyAbsolute(-12345)).toContain('123,45');
  });

  it('formats compact currency without symbol for dense reports', () => {
    expect(centsToCompactCurrencyAbsolute(100000000)).toBe('1.000.000,00');
  });
});
