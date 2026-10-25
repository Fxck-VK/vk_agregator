import {deviceBookmarks} from "@/components/public/reference/device-bookmarks";
const bookmarks=deviceBookmarks("epifera.saved-collections");
export const isCollectionSaved=bookmarks.isSaved;
export const toggleCollectionSaved=bookmarks.toggle;
export const subscribeCollections=bookmarks.subscribe;
