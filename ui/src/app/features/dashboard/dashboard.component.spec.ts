import { TestBed } from '@angular/core/testing';
import { of, Subject } from 'rxjs';
import { vi } from 'vitest';

import { ReferenceDataService } from '../../data/reference-data.service';
import { ReportsService } from '../../data/reports.service';
import { DashboardComponent } from './dashboard.component';

describe('DashboardComponent', () => {
  const dashboardData = {
    year: 2026,
    month: 1,
    balances: [{ AccountCode: 'santander', Balance: 12345 }],
    yearly: [
      { code: 'salario', monthly_data: [300000, 300000, 300000, 300000, 300000, 300000, 300000, 300000, 300000, 300000, 300000, 300000], subcategories: [] },
      { code: 'alimentacao', monthly_data: [-10000, -10000, -10000, -10000, -10000, -10000, -10000, -10000, -10000, -10000, -10000, -14000], subcategories: [] },
      { code: 'moradia', monthly_data: [-20000, -20000, -20000, -20000, -20000, -20000, -20000, -20000, -20000, -20000, -20000, -21000], subcategories: [] },
      { code: 'transporte', monthly_data: [-7000, -7000, -7000, -7000, -7000, -7000, -7000, -7000, -9000, -10000, -11000, -7000], subcategories: [] },
      { code: 'lazer', monthly_data: [0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, -5000], subcategories: [] },
      { code: 'zerada', monthly_data: [0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0], subcategories: [] },
    ],
    recent_transactions: [
      {
        id: '1',
        category_code: 'alimentacao',
        description: 'Mercado',
        date: '2026-01-02T00:00:00Z',
        account_code: 'santander',
        amount: -5000,
      },
    ],
    top_expenses: [
      {
        id: '1',
        category_code: 'alimentacao',
        description: 'Mercado',
        date: '2026-01-02T00:00:00Z',
        account_code: 'santander',
        amount: -5000,
      },
    ],
  };
  const dashboard = vi.fn();

  beforeEach(async () => {
    dashboard.mockReset();
    dashboard.mockReturnValue(of(dashboardData));
    await TestBed.configureTestingModule({
      imports: [DashboardComponent],
      providers: [
        {
          provide: ReportsService,
          useValue: {
            dashboard,
          },
        },
        {
          provide: ReferenceDataService,
          useValue: {
            load: () => of(void 0),
            accountName: (code: string) => (code === 'santander' ? 'Santander' : code),
            categoryName: (code: string) => ({
              salario: 'Salario',
              alimentacao: 'Alimentacao',
              moradia: 'Moradia',
              transporte: 'Transporte',
              lazer: 'Lazer',
              zerada: 'Zerada',
            })[code] ?? code,
            flatCategories: () => [
              { Code: 'salario', Type: 'INCOME' },
              { Code: 'alimentacao', Type: 'EXPENSE' },
              { Code: 'moradia', Type: 'EXPENSE' },
              { Code: 'transporte', Type: 'EXPENSE' },
              { Code: 'lazer', Type: 'EXPENSE' },
              { Code: 'zerada', Type: 'EXPENSE' },
            ],
            accounts: () => [
              { Code: 'santander', Name: 'Santander', Type: 'ASSET', hide_from_dashboard: false },
              { Code: 'btg', Name: 'BTG', Type: 'ASSET', hide_from_dashboard: true },
            ],
          },
        },
      ],
    }).compileComponents();
  });

  it('renders dashboard data and the category benchmark card', async () => {
    const fixture = TestBed.createComponent(DashboardComponent);
    fixture.detectChanges();
    await fixture.whenStable();
    fixture.detectChanges();

    const compiled = fixture.nativeElement as HTMLElement;
    expect(compiled.textContent).toContain('Painel');
    expect(compiled.textContent).toContain('Fluxo 12m');
    expect(compiled.textContent).toContain('Santander');
    expect(compiled.textContent).toContain('Mercado');
    expect(compiled.textContent).not.toContain('BTG');
    expect(compiled.textContent).toContain('Comparativo de gastos recentes');
    expect(compiled.textContent).toContain('Moradia');
    expect(compiled.textContent).toContain('Alimentacao');
    expect(compiled.textContent).toContain('Transporte');
    expect(compiled.textContent).toContain('Lazer');
    expect(compiled.textContent).not.toContain('Zerada');
    expect(compiled.textContent).toContain('acima da média');
    expect(compiled.textContent).toContain('na média');
    expect(compiled.textContent).toContain('abaixo da média');
  });

  it('computes category benchmark rows with status, zero-baseline inclusion, and ordering', async () => {
    const fixture = TestBed.createComponent(DashboardComponent);
    fixture.detectChanges();
    await fixture.whenStable();

    const rows = fixture.componentInstance.categoryBenchmarkRows();
    expect(rows.map((row) => row.code)).toEqual(['moradia', 'alimentacao', 'transporte', 'lazer']);

    expect(rows.find((row) => row.code === 'moradia')).toMatchObject({
      current: 21000,
      baseline: 20000,
      status: 'near',
    });
    expect(rows.find((row) => row.code === 'alimentacao')).toMatchObject({
      current: 14000,
      baseline: 10000,
      status: 'above',
    });
    expect(rows.find((row) => row.code === 'transporte')).toMatchObject({
      current: 7000,
      baseline: 8000,
      status: 'below',
    });
    expect(rows.find((row) => row.code === 'lazer')).toMatchObject({
      current: 5000,
      baseline: 0,
      status: 'above',
      deltaPercent: null,
    });
  });

  it('allows the benchmark with only one prior month of expense data', async () => {
    dashboard.mockReturnValueOnce(
      of({
        ...dashboardData,
        yearly: [
          { code: 'salario', monthly_data: [0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 300000, 300000], subcategories: [] },
          { code: 'alimentacao', monthly_data: [0, 0, 0, 0, 0, 0, 0, 0, 0, 0, -12000, -9000], subcategories: [] },
        ],
      }),
    );

    const fixture = TestBed.createComponent(DashboardComponent);
    fixture.detectChanges();
    await fixture.whenStable();
    fixture.detectChanges();

    expect(fixture.componentInstance.categoryBenchmarkRows()).toHaveLength(1);
    expect(fixture.nativeElement.textContent).not.toContain('Sem dados em meses anteriores');
  });

  it('shows a history-unavailable state when there is no prior expense data', async () => {
    dashboard.mockReturnValueOnce(
      of({
        ...dashboardData,
        yearly: [
          { code: 'salario', monthly_data: [0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 300000], subcategories: [] },
          { code: 'alimentacao', monthly_data: [0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, -9000], subcategories: [] },
        ],
      }),
    );

    const fixture = TestBed.createComponent(DashboardComponent);
    fixture.detectChanges();
    await fixture.whenStable();
    fixture.detectChanges();

    expect(fixture.nativeElement.textContent).toContain('Sem dados em meses anteriores para comparar a mediana recente.');
  });

  it('shows a dashboard skeleton while loading and disables month controls', async () => {
    const response$ = new Subject<typeof dashboardData>();
    dashboard.mockReturnValueOnce(response$.asObservable());

    const fixture = TestBed.createComponent(DashboardComponent);
    fixture.detectChanges();

    const host = fixture.nativeElement as HTMLElement;
    const previousButton = host.querySelector('button[aria-label="Mês anterior"]') as HTMLButtonElement;
    const nextButton = host.querySelector('button[aria-label="Mês seguinte"]') as HTMLButtonElement;

    expect(host.querySelector('[data-testid="dashboard-skeleton"]')).not.toBeNull();
    expect(previousButton.disabled).toBe(true);
    expect(nextButton.disabled).toBe(true);

    response$.next(dashboardData);
    response$.complete();
    await fixture.whenStable();
    fixture.detectChanges();

    expect(host.querySelector('[data-testid="dashboard-skeleton"]')).toBeNull();
    expect(host.textContent).toContain('Fluxo 12m');
  });

  it('loads the previous month when navigating backward', async () => {
    const fixture = TestBed.createComponent(DashboardComponent);
    fixture.detectChanges();
    await fixture.whenStable();
    fixture.detectChanges();

    dashboard.mockClear();
    fixture.componentInstance.goToPreviousMonth();

    expect(dashboard).toHaveBeenCalledWith(2025, 12);
  });
});
