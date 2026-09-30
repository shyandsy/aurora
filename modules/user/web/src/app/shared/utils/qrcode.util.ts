import QRCode from 'qrcode';

/**
 * 把 otpauth:// URI 渲染成一张 PNG data URL(离线内置,不引任何 CDN)。
 * 用在两步验证绑定页:<img [src]="dataUrl"> 直接显示二维码,供 Google Authenticator 扫描。
 */
export function otpauthToQrDataUrl(otpauthUri: string): Promise<string> {
  return QRCode.toDataURL(otpauthUri, {
    errorCorrectionLevel: 'M',
    margin: 2,
    width: 220,
  });
}
