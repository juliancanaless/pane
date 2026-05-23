class Pane < Formula
  desc "Shared local memory and coordination for concurrent coding agents"
  homepage "https://github.com/juliancanaless/pane"
  version "0.1.1"
  license "MIT"
  head "https://github.com/juliancanaless/pane.git", branch: "main"

  on_macos do
    if Hardware::CPU.arm?
      url "https://github.com/juliancanaless/pane/releases/download/v0.1.1/pane-v0.1.1-darwin-arm64.tar.gz"
      sha256 "3717f29b2cf2f4f0e309322c6a290a50d8a470ee9c7fcefa40ea5fb3d0e8c97c"
    else
      url "https://github.com/juliancanaless/pane/releases/download/v0.1.1/pane-v0.1.1-darwin-amd64.tar.gz"
      sha256 "0dc5631eb829677d3456b468e2990b1c18ea2b4fc115e3916abaf40626e673ee"
    end
  end

  depends_on "go" => :build if build.head?
  depends_on "rust" => :build if build.head?

  def install
    if build.head?
      ENV.prepend_path "PATH", Formula["rust"].opt_bin
      system "make", "build"
      bin.install "bin/pane"
      bin.install "bin/pane-analyze"
    else
      bin.install "pane"
      bin.install "pane-analyze"
    end
  end

  test do
    assert_match "Pane gives concurrent coding agents", shell_output("#{bin}/pane help")
    assert_match "platform:", shell_output("#{bin}/pane doctor")
  end
end
