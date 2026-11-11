/**
 * The one store of app state. Each stores/ module owns one collection and
 * gives typed functions over it. An element watches a collection
 * (`static watches`) and renders again when it changes.
 */
import BasicDataService from "@catalystcommunity/basic-data-service";

export const store = new BasicDataService();

export interface Cell<T> {
  get(): T;
  set(value: T): void;
}

/** One value at `collection`/`key`. A new value must be a new object. */
export function cell<T>(collection: string, key: string, initial: T): Cell<T> {
  store.set(collection, key, initial);
  return {
    get: () => store.get(collection, key) as T,
    set: (value: T) => store.set(collection, key, value),
  };
}
