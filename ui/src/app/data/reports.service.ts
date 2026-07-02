import { HttpClient, HttpParams } from '@angular/common/http';
import { Injectable } from '@angular/core';
import { map } from 'rxjs';

import { AccountBalance, CategoryYearlyBalance, DashboardReport } from '../shared/models';

@Injectable({ providedIn: 'root' })
export class ReportsService {
  constructor(private readonly http: HttpClient) {}

  balances() {
    return this.http.get<{ accounts: AccountBalance[] }>('/api/reports/balance').pipe(map((res) => res.accounts));
  }

  yearly(year = new Date().getFullYear()) {
    const params = new HttpParams().set('year', year);
    return this.http.get<{ balances: CategoryYearlyBalance[] }>('/api/reports/yearly', { params }).pipe(map((res) => res.balances));
  }

  dashboard(year = new Date().getFullYear(), month = new Date().getMonth() + 1) {
    const params = new HttpParams().set('year', year).set('month', month);
    return this.http.get<{ dashboard: DashboardReport }>('/api/reports/dashboard', { params }).pipe(map((res) => res.dashboard));
  }
}
