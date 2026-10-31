import type {PaymentProvider} from "./provider.js";
import {FamGatewayProvider} from "./famgateway.js";
const providers=new Map<string,PaymentProvider>([["famgateway",new FamGatewayProvider()]]);
export function getPaymentProvider(id:string){const provider=providers.get(id);if(!provider)throw new Error("Payment provider is not enabled");return provider;}
export function listPaymentProviders(){return [...providers.keys()];}
