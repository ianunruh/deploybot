import type { Route } from "./+types/deployables.$name.images";
import { listImages, type ImageList } from "~/lib/api.server";

export type ImagesLoaderData = ImageList & { error: string | null };

export async function loader({ params }: Route.LoaderArgs): Promise<ImagesLoaderData> {
  const name = params.name;
  if (!name) {
    throw new Response("Missing name", { status: 400 });
  }
  try {
    const data = await listImages(name);
    return { ...data, error: null };
  } catch (err) {
    return {
      repository: "",
      source: "",
      images: [],
      error: err instanceof Error ? err.message : String(err),
    };
  }
}
