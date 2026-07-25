import { TestBed } from '@angular/core/testing';
import { provideRouter } from '@angular/router';
import { of } from 'rxjs';

import { InvestmentsService } from '../../data/investments.service';
import { InvestmentDashboardComponent } from './investment-dashboard.component';

describe('InvestmentDashboardComponent', () => {
  beforeEach(async () => {
    await TestBed.configureTestingModule({
      imports: [InvestmentDashboardComponent],
      providers: [
        provideRouter([]),
        {
          provide: InvestmentsService,
          useValue: {
            listPositions: () => of([
              {
                asset_code: 'PETR4', asset_name: 'Petrobras', asset_type: 'STOCK', portfolio_names: ['Brasil'],
                current_quantity: 10, average_price: 2000, total_cost_basis: 20000, realized_pnl: 500, matched_dividends_total: 300, last_recalculated: '2026-07-14T00:00:00Z',
              },
              {
                asset_code: 'XPLG11', asset_name: 'XPLG', asset_type: 'FII', portfolio_names: ['Renda'],
                current_quantity: 5, average_price: 10000, total_cost_basis: 50000, realized_pnl: -200, matched_dividends_total: 1000, last_recalculated: '2026-07-14T00:00:00Z',
              },
            ]),
            listPositionQuotes: () => of([
              { asset_code: 'PETR4', current_price: 2500, quote_updated_at: '2026-07-14T12:00:00Z' },
              { asset_code: 'XPLG11', current_price: 9000, quote_updated_at: '2026-07-14T12:00:00Z' },
            ]),
          },
        },
      ],
    }).compileComponents();
  });

  it('aggregates market value and investment results from positions and quotes', async () => {
    const fixture = TestBed.createComponent(InvestmentDashboardComponent);
    fixture.detectChanges();
    await fixture.whenStable();
    fixture.detectChanges();

    const component = fixture.componentInstance;
    expect(component.totalMarketValue()).toBe(70000);
    expect(component.totalCostBasis()).toBe(70000);
    expect(component.totalUnrealizedPnl()).toBe(0);
    expect(component.totalRealizedPnl()).toBe(300);
    expect(component.totalDividends()).toBe(1300);
    expect(component.totalPnl()).toBe(1600);
    expect(component.allocationRows().map((row) => row.type)).toEqual(['FII', 'STOCK']);
    expect(component.topHoldings().map((position) => position.asset_code)).toEqual(['XPLG11', 'PETR4']);
  });
});
