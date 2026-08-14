class Pane < Formula
  desc "Shared local memory and coordination for concurrent coding agents"
  homepage "https://github.com/juliancanaless/pane"
  version "0.1.7"
  license "MIT"
  head "https://github.com/juliancanaless/pane.git", branch: "main"

  on_macos do
    if Hardware::CPU.arm?
      url "https://github.com/juliancanaless/pane/releases/download/v0.1.7/pane-v0.1.7-darwin-arm64.tar.gz"
      sha256 "1b33c30c28181946bfb8af2f3cd32c00d4df028a52b620d5f01782d3a4d487d0"
    else
      url "https://github.com/juliancanaless/pane/releases/download/v0.1.7/pane-v0.1.7-darwin-amd64.tar.gz"
      sha256 "0504001117eb98156c8a95429aa52dd6a87c1b42af6f0ed5a828bb93c7348479"
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
