import { Composition } from "remotion";
import { MainComposition } from "./MainComposition";

// 24 seconds @ 30fps = 720 frames
export const RemotionRoot = () => {
  return (
    <Composition
      id="MainComposition"
      component={MainComposition}
      durationInFrames={720}
      fps={30}
      width={1920}
      height={1080}
    />
  );
};
