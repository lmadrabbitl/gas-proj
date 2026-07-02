import { TestBed } from '@angular/core/testing';
import { provideRouter, Router, UrlTree } from '@angular/router';
import { firstValueFrom, Observable, of } from 'rxjs';

import { AuthService } from './auth.service';
import { authGuard } from './auth.guard';
import { ReferenceDataService } from '../data/reference-data.service';
import { UserConfigService } from '../data/user-config.service';

describe('authGuard', () => {
  let loggedIn = false;

  beforeEach(() => {
    loggedIn = false;
    localStorage.clear();
    TestBed.configureTestingModule({
      providers: [
        provideRouter([]),
        {
          provide: AuthService,
          useValue: {
            isLoggedIn: () => loggedIn,
            logout: () => undefined,
          },
        },
        {
          provide: ReferenceDataService,
          useValue: {
            load: () => of(void 0),
            clear: () => undefined,
          },
        },
        {
          provide: UserConfigService,
          useValue: {
            load: () => of({}),
            clear: () => undefined,
          },
        },
      ],
    });
  });

  afterEach(() => localStorage.clear());

  it('redirects anonymous users to login', () => {
    const result = TestBed.runInInjectionContext(() => authGuard({} as never, {} as never));

    expect(result instanceof UrlTree).toBe(true);
    expect(TestBed.inject(Router).serializeUrl(result as UrlTree)).toBe('/login');
  });

  it('allows logged in users', async () => {
    loggedIn = true;

    const result = TestBed.runInInjectionContext(() => authGuard({} as never, {} as never));

    expect(await firstValueFrom(result as Observable<boolean | UrlTree>)).toBe(true);
  });
});
