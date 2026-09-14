import { isCheckoutURL } from "./payments";

export const checkoutNavigation = {
  open(url: string) {
    if (!isCheckoutURL(url)) throw new Error("Invalid checkout URL");
    window.location.assign(url);
  },
};
