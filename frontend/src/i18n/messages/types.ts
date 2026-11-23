/** A dictionary shape with every leaf widened to string. */
export type DeepString<T> = {
    [K in keyof T]: T[K] extends string ? string : DeepString<T[K]>;
};
