import { HttpInterceptorFn } from '@angular/common/http';
import { inject } from '@angular/core';
import { Router } from '@angular/router';
import { catchError, throwError } from 'rxjs';

import { AuthService } from './auth.service';
import { ReferenceDataService } from '../data/reference-data.service';
import { ToastService } from '../shared/toast.service';
import { UserConfigService } from '../data/user-config.service';

export const authInterceptor: HttpInterceptorFn = (req, next) => {
  const auth = inject(AuthService);
  const router = inject(Router);
  const referenceData = inject(ReferenceDataService);
  const toast = inject(ToastService);
  const userConfig = inject(UserConfigService);
  const token = auth.token();

  const authReq = token
    ? req.clone({
        setHeaders: {
          Authorization: `Bearer ${token}`,
        },
      })
    : req;

  return next(authReq).pipe(
    catchError((error) => {
      const isPublicAuthRequest =
        req.url.includes('/api/auth/login') || req.url.includes('/api/auth/register');

      if (error.status === 401 && !isPublicAuthRequest) {
        auth.logout();
        referenceData.clear();
        toast.clear();
        userConfig.clear();
        void router.navigate(['/login']);
      }
      return throwError(() => error);
    }),
  );
};
