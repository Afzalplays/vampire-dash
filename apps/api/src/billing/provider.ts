export type CreatePaymentInput={orderId:string;amountPaise:number;currency:"INR";customer:{name?:string;email?:string;phone?:string};webhookUrl:string;redirectUrl:string};
export type ProviderOrder={providerOrderId:string;checkoutUrl?:string;qrUrl?:string;expiresAt?:string};
export type VerifiedPayment={providerOrderId:string;status:"PAID"|"PENDING"|"FAILED";amountPaise:number;currency:string;transactionId?:string};
export interface PaymentProvider {readonly id:string;createPayment(input:CreatePaymentInput):Promise<ProviderOrder>;verifyPayment(providerOrderId:string):Promise<VerifiedPayment>;verifyWebhook(rawBody:Buffer,signature:string):boolean;}
