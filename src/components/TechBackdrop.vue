<template>
  <div class="tech-bg" aria-hidden="true">
    <div class="tech-bg__mesh" />
    <div class="tech-bg__grid" />
    <div class="tech-bg__orb tech-bg__orb--1" />
    <div class="tech-bg__orb tech-bg__orb--2" />
    <div class="tech-bg__orb tech-bg__orb--3" />
    <div class="tech-bg__scan" />
  </div>
</template>

<style scoped>
.tech-bg {
  position: fixed;
  inset: 0;
  z-index: 0;
  pointer-events: none;
  overflow: hidden;
}

.tech-bg__mesh {
  position: absolute;
  inset: -20%;
  animation: tech-mesh 18s ease-in-out infinite alternate;
}

.tech-bg__grid {
  position: absolute;
  inset: 0;
  background-image:
    linear-gradient(rgba(148, 163, 184, 0.08) 1px, transparent 1px),
    linear-gradient(90deg, rgba(148, 163, 184, 0.08) 1px, transparent 1px);
  background-size: 48px 48px;
  mask-image: radial-gradient(ellipse 80% 70% at 50% 40%, #000 20%, transparent 75%);
  animation: tech-grid 24s linear infinite;
}

.tech-bg__orb {
  position: absolute;
  border-radius: 50%;
  filter: blur(40px);
  opacity: 0.55;
  animation: tech-orb 12s ease-in-out infinite;
}

.tech-bg__orb--1 {
  width: 280px;
  height: 280px;
  top: 12%;
  left: 8%;
  background: rgba(34, 211, 238, 0.35);
}

.tech-bg__orb--2 {
  width: 360px;
  height: 360px;
  right: 5%;
  top: 35%;
  background: rgba(124, 58, 237, 0.28);
  animation-delay: -4s;
}

.tech-bg__orb--3 {
  width: 220px;
  height: 220px;
  left: 42%;
  bottom: 8%;
  background: rgba(59, 130, 246, 0.25);
  animation-delay: -7s;
}

.tech-bg__scan {
  position: absolute;
  inset: 0;
  background: repeating-linear-gradient(
    0deg,
    transparent,
    transparent 2px,
    rgba(255, 255, 255, 0.015) 2px,
    rgba(255, 255, 255, 0.015) 4px
  );
  opacity: 0.35;
}

@keyframes tech-mesh {
  0% {
    transform: scale(1) translate(0, 0);
  }
  100% {
    transform: scale(1.05) translate(2%, -1%);
  }
}

@keyframes tech-grid {
  0% {
    transform: perspective(500px) rotateX(58deg) translateY(0);
  }
  100% {
    transform: perspective(500px) rotateX(58deg) translateY(48px);
  }
}

@keyframes tech-orb {
  0%,
  100% {
    transform: translate(0, 0) scale(1);
  }
  50% {
    transform: translate(12px, -18px) scale(1.06);
  }
}
</style>
