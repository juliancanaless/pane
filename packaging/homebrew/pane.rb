class Pane < Formula
  desc "Shared local memory and coordination for concurrent coding agents"
  homepage "https://github.com/juliancanaless/pane"
  version "0.1.6"
  license "MIT"
  head "https://github.com/juliancanaless/pane.git", branch: "main"

  on_macos do
    if Hardware::CPU.arm?
      url "https://github.com/juliancanaless/pane/releases/download/v0.1.6/pane-v0.1.6-darwin-arm64.tar.gz"
      sha256 "0b06a9a878ad8536d2c21b89165aed57da2bd76bbe3f67490520575503135b27"
    else
      url "https://github.com/juliancanaless/pane/releases/download/v0.1.6/pane-v0.1.6-darwin-amd64.tar.gz"
      sha256 "f9f8ce0545dbde3e583d34baa3382624c4590207024eb2b8410d9c26fa831d3c"
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
