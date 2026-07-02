import { provideHttpClient, withInterceptors } from '@angular/common/http';
import { HttpClient, HttpErrorResponse } from '@angular/common/http';
import { provideHttpClientTesting, HttpTestingController } from '@angular/common/http/testing';
import { TestBed } from '@angular/core/testing';
import { provideRouter } from '@angular/router';

import { AuthService } from './auth.service';
import { authInterceptor } from './auth.interceptor';
import { ReferenceDataService } from '../data/reference-data.service';
import { UserConfigService } from '../data/user-config.service';

describe('authInterceptor', () => {
  let http: HttpTestingController;
  let client: HttpClient;
  let auth: AuthService;

  beforeEach(() => {
    localStorage.clear();
    TestBed.configureTestingModule({
      providers: [
        provideRouter([]),
        provideHttpClient(withInterceptors([authInterceptor])),
        provideHttpClientTesting(),
        {
          provide: ReferenceDataService,
          useValue: {
            clear: () => undefined,
          },
        },
        {
          provide: UserConfigService,
          useValue: {
            clear: () => undefined,
          },
        },
      ],
    });
    http = TestBed.inject(HttpTestingController);
    client = TestBed.inject(HttpClient);
    auth = TestBed.inject(AuthService);
  });

  afterEach(() => {
    http.verify();
    localStorage.clear();
  });

  it('adds the bearer token to API calls', () => {
    auth.login('test@user.com', 'password123').subscribe();
    http.expectOne('/api/auth/login').flush({ token: 'abc123' });

    client.get('/api/accounts').subscribe();

    const req = http.expectOne('/api/accounts');
    expect(req.request.headers.get('Authorization')).toBe('Bearer abc123');
    req.flush({ accounts: [] });
  });

  it('does not clear auth state on public auth 401 responses', () => {
    let capturedError: HttpErrorResponse | undefined;

    client.post('/api/auth/login', { email: 'test@user.com', password: 'bad-password' }).subscribe({
      error: (error) => {
        capturedError = error;
      },
    });

    const req = http.expectOne('/api/auth/login');
    req.flush({ error: { code: 'auth.login.invalid_credentials', error: 'invalid login/password' } }, { status: 401, statusText: 'Unauthorized' });

    expect(capturedError?.status).toBe(401);
    expect(auth.token()).toBeNull();
    expect(auth.isLoggedIn()).toBe(false);
  });

  it('does not clear auth state on protected api 500 responses', () => {
    let capturedError: HttpErrorResponse | undefined;

    auth.login('test@user.com', 'password123').subscribe();
    http.expectOne('/api/auth/login').flush({ token: 'abc123' });

    client.post('/api/investments/assets/CCRO3/refresh-metadata', {}).subscribe({
      error: (error) => {
        capturedError = error;
      },
    });

    const req = http.expectOne('/api/investments/assets/CCRO3/refresh-metadata');
    req.flush({ error: { error: 'metadata not found' } }, { status: 500, statusText: 'Internal Server Error' });

    expect(capturedError?.status).toBe(500);
    expect(auth.token()).toBe('abc123');
    expect(auth.isLoggedIn()).toBe(true);
  });
});
