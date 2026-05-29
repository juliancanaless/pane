class Pane < Formula
  desc "Shared local memory and coordination for concurrent coding agents"
  homepage "https://github.com/juliancanaless/pane"
  version "0.1.3"
  license "MIT"
  head "https://github.com/juliancanaless/pane.git", branch: "main"

  on_macos do
    if Hardware::CPU.arm?
      url "https://github.com/juliancanaless/pane/releases/download/v0.1.3/pane-v0.1.3-darwin-arm64.tar.gz"
      sha256 "65f3cba3829e15aaf5505e8d324343072188c33ba59f338af6b9572f3effe6c8"
    else
      url "https://github.com/juliancanaless/pane/releases/download/v0.1.3/pane-v0.1.3-darwin-amd64.tar.gz"
      sha256 "fd04d61767db6d3b83b23dc257ecdb39a8283620284209c7de3e26eb1d612080"
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
